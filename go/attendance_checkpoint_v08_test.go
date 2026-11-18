//go:build linux && cgo

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestV08RawPunchImmutabilityAndIdempotentMigration(t *testing.T) {
	v := secured(t)
	if err := attendanceSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := seedAttendanceDemo(v.service.db); err != nil { t.Fatal(err) }
	for n := 0; n < 2; n++ {
		if err := attendanceCheckpointSchema(v.service.db); err != nil { t.Fatal(err) }
	}
	original, err := v.service.one(`SELECT direction FROM oa_raw_punch
		WHERE tenant_id=? AND source='SIMULATOR' AND device_id='SIM-GATE-01'
		AND event_id='SIM-0012-02-IN'`, tenantID)
	if err != nil || original == nil { t.Fatalf("missing fixture: %v", err) }
	if err := v.service.db.Exec(`UPDATE oa_raw_punch SET direction='OUT'
		WHERE tenant_id=? AND event_id='SIM-0012-02-IN'`, tenantID); err == nil {
		t.Fatal("raw UPDATE accepted")
	}
	if err := v.service.db.Exec(`DELETE FROM oa_raw_punch
		WHERE tenant_id=? AND event_id='SIM-0012-02-IN'`, tenantID); err == nil {
		t.Fatal("raw DELETE accepted")
	}
	after, err := v.service.one(`SELECT direction FROM oa_raw_punch WHERE tenant_id=? AND event_id='SIM-0012-02-IN'`, tenantID)
	if err != nil || after == nil || rowString(after, "direction") != rowString(original, "direction") {
		t.Fatalf("source event changed: row=%v err=%v", after, err)
	}
}

func TestV08DeviceCursorAtomicReplayAndOffline(t *testing.T) {
	v := secured(t)
	if err := attendanceSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := attendanceCheckpointSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := seedAttendanceDemo(v.service.db); err != nil { t.Fatal(err) }
	var phase atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cursor := r.URL.Query().Get("cursor")
		switch phase.Load() {
		case 0:
			if cursor != "" { t.Errorf("initial cursor=%q", cursor) }
			fmt.Fprint(w, `{"events":[{"event_id":"edge-1","badge_id":"EMP-0012","direction":"IN","occurred_at":"2026-11-08T08:00:00+07:00"},{"event_id":"edge-unknown","badge_id":"UNKNOWN-77","direction":"OUT","occurred_at":"2026-11-08T16:00:00+07:00"}],"next_cursor":"offset-2"}`)
		case 1:
			if cursor != "offset-2" { t.Errorf("checkpoint not loaded: %q", cursor) }
			fmt.Fprint(w, `{"events":[{"event_id":"edge-2","badge_id":"EMP-0012","direction":"OUT","occurred_at":"2026-11-08T16:00:00+07:00"},{"event_id":"edge-1","badge_id":"EMP-0012","direction":"OUT","occurred_at":"2026-11-08T08:00:00+07:00"}],"next_cursor":"offset-4"}`)
		case 2:
			if cursor != "offset-2" { t.Errorf("rollback cursor=%q", cursor) }
			fmt.Fprint(w, `{"events":[{"event_id":"edge-2","badge_id":"EMP-0012","direction":"OUT","occurred_at":"2026-11-08T16:00:00+07:00"},{"event_id":"edge-unknown","badge_id":"UNKNOWN-77","direction":"OUT","occurred_at":"2026-11-08T16:00:00+07:00"}],"next_cursor":"offset-4"}`)
		case 3:
			if cursor != "offset-4" { t.Errorf("idle cursor=%q", cursor) }
			fmt.Fprint(w, `{"events":[],"next_cursor":"offset-4"}`)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	t.Setenv("ORGAXIS_DEVICE_ENDPOINT", server.URL)
	t.Setenv("ORGAXIS_DEVICE_SOURCE", "EDGE-JSON")
	t.Setenv("ORGAXIS_DEVICE_ID", "SIM-GATE-01")
	t.Setenv("ORGAXIS_DEVICE_TOKEN", "")
	hr := signin(t, v, "hr")
	employee := signin(t, v, "employee")
	if x := call(v.service, "POST", "/v2/time/adapter/pull", "{}", employee, true); x.Code != 403 {
		t.Fatalf("employee pulled device page: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, false); x.Code != 403 {
		t.Fatalf("missing CSRF allowed device pull: %d", x.Code)
	}
	x := call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, true)
	if x.Code != 201 { t.Fatalf("page 1: %d %s", x.Code, x.Body) }
	first := obj(t, x.Body.Bytes())
	if int(first["accepted"].(float64)) != 1 || int(first["exceptions"].(float64)) != 1 {
		t.Fatalf("unexpected page 1 result: %v", first)
	}
	state, err := v.service.one(`SELECT cursor FROM oa_time_pull_checkpoint WHERE tenant_id=? AND source='EDGE-JSON' AND device_id='SIM-GATE-01'`, tenantID)
	if err != nil || rowString(state, "cursor") != "offset-2" { t.Fatalf("cursor not persisted: %v %v", state, err) }
	phase.Store(1)
	x = call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, true)
	if x.Code != 409 { t.Fatalf("conflicting page accepted: %d %s", x.Code, x.Body) }
	state, err = v.service.one(`SELECT cursor FROM oa_time_pull_checkpoint WHERE tenant_id=? AND source='EDGE-JSON'`, tenantID)
	if err != nil || rowString(state, "cursor") != "offset-2" { t.Fatalf("cursor advanced on rollback: %v %v", state, err) }
	notCommitted, err := v.service.one(`SELECT event_id FROM oa_raw_punch WHERE tenant_id=? AND source='EDGE-JSON' AND event_id='edge-2'`, tenantID)
	if err != nil || notCommitted != nil { t.Fatalf("partial page committed: %v %v", notCommitted, err) }
	phase.Store(2)
	x = call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, true)
	if x.Code != 201 { t.Fatalf("replay page: %d %s", x.Code, x.Body) }
	res := obj(t, x.Body.Bytes())
	if int(res["accepted"].(float64)) != 1 || int(res["duplicate"].(float64)) != 1 {
		t.Fatalf("replay mismatch: %v", res)
	}
	state, err = v.service.one(`SELECT cursor FROM oa_time_pull_checkpoint WHERE tenant_id=? AND source='EDGE-JSON'`, tenantID)
	if err != nil || rowString(state, "cursor") != "offset-4" { t.Fatalf("replay cursor missing: %v %v", state, err) }
	phase.Store(3)
	x = call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, true)
	if x.Code != 201 || obj(t, x.Body.Bytes())["idle"] != true {
		t.Fatalf("empty page not idle: %d %s", x.Code, x.Body)
	}
	phase.Store(4)
	x = call(v.service, "POST", "/v2/time/adapter/pull", "{}", hr, true)
	if x.Code != 502 { t.Fatalf("offline did not fail safely: %d %s", x.Code, x.Body) }
	state, err = v.service.one(`SELECT cursor FROM oa_time_pull_checkpoint WHERE tenant_id=? AND source='EDGE-JSON'`, tenantID)
	if err != nil || rowString(state, "cursor") != "offset-4" { t.Fatalf("offline advanced cursor: %v %v", state, err) }
	if x := call(v.service, "GET", "/v2/time/adapter/state", "", employee, false); x.Code != 403 {
		t.Fatalf("employee read device checkpoint: %d", x.Code)
	}
	if x := call(v.service, "GET", "/v2/time/adapter/state", "", hr, false); x.Code != 200 {
		t.Fatalf("HR cannot inspect checkpoint: %d %s", x.Code, x.Body)
	}
}

func TestV08EffectiveBadgeDateUsesVietnamSiteTimezone(t *testing.T) {
	v := secured(t)
	if err := attendanceSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := attendanceCheckpointSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := seedAttendanceDemo(v.service.db); err != nil { t.Fatal(err) }
	// 2026-11-08 17:30 UTC is 2026-11-09 00:30 at the configured VN plant.
	// The replacement badge assignment begins only on Nov 9 local time.
	if err := v.service.db.Exec(`UPDATE oa_badge_binding SET valid_to='2026-11-09'
		WHERE tenant_id=? AND device_id='SIM-GATE-01' AND badge_id='EMP-0012'`, tenantID); err != nil {
		t.Fatal(err)
	}
	if err := v.service.db.Exec(`INSERT INTO oa_badge_binding
		(tenant_id,device_id,badge_id,person_id,valid_from) VALUES(?,?,?,?,?)`,
		tenantID, "SIM-GATE-01", "EMP-0012", "EMP-0013", "2026-11-09"); err != nil { t.Fatal(err) }
	hr := signin(t, v, "hr")
	body := `{"source":"UTC-GATEWAY","device_id":"SIM-GATE-01","events":[{"event_id":"utc-crossing","badge_id":"EMP-0012","direction":"IN","occurred_at":"2026-11-08T17:30:00Z"}]}`
	resp := call(v.service, "POST", "/v2/time/punches/import", body, hr, true)
	if resp.Code != 201 { t.Fatalf("timezone import: %d %s", resp.Code, resp.Body) }
	row, err := v.service.one(`SELECT person_id FROM oa_raw_punch
		WHERE tenant_id=? AND source='UTC-GATEWAY' AND event_id='utc-crossing'`, tenantID)
	if err != nil || rowString(row, "person_id") != "EMP-0013" {
		t.Fatalf("wrong effective-date badge binding: row=%v err=%v", row, err)
	}
}

func TestV08ExistingTimeDeviceSchemaUpgradesWithoutReplacement(t *testing.T) {
	v := secured(t)
	if err := v.service.db.Exec(`CREATE TABLE oa_time_device (
		tenant_id TEXT NOT NULL,device_id TEXT NOT NULL,site_id TEXT NOT NULL,
		description TEXT NOT NULL,source_type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,PRIMARY KEY(tenant_id,device_id))`); err != nil { t.Fatal(err) }
	if err := v.service.db.Exec(`INSERT INTO oa_time_device VALUES(?,?,?,?,?,?)`,
		tenantID, "OLD-DEVICE", "TKP", "existing device", "MANUAL", 1); err != nil { t.Fatal(err) }
	if err := attendanceSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := attendanceCheckpointSchema(v.service.db); err != nil { t.Fatal(err) }
	if err := attendanceCheckpointSchema(v.service.db); err != nil { t.Fatal(err) }
	row, err := v.service.one(`SELECT description,timezone FROM oa_time_device
		WHERE tenant_id=? AND device_id='OLD-DEVICE'`, tenantID)
	if err != nil || rowString(row, "timezone") != "Asia/Ho_Chi_Minh" ||
		rowString(row, "description") != "existing device" {
		t.Fatalf("existing device was not preserved: row=%v err=%v", row, err)
	}
}
