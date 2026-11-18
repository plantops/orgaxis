//go:build linux && cgo

package main

import (
	"encoding/json"
	"testing"
)

func TestAttendanceSeedAndScopedAPIs(t *testing.T) {
	v := secured(t)
	if err := attendanceSchema(v.service.db); err != nil {
		t.Fatal(err)
	}
	if err := seedAttendanceDemo(v.service.db); err != nil {
		t.Fatal(err)
	}
	// Repeated startup must never overwrite imported attendance or duplicate seed.
	if err := seedAttendanceDemo(v.service.db); err != nil {
		t.Fatal(err)
	}
	emp := signin(t, v, "employee")
	hr := signin(t, v, "hr")
	admin := signin(t, v, "admin")
	w := call(v.service, "GET", "/v2/time/day?date=2026-11-02", "", emp, false)
	if w.Code != 200 {
		t.Fatalf("employee timecard: %d %s", w.Code, w.Body)
	}
	if obj(t, w.Body.Bytes())["person_id"] != "EMP-0012" {
		t.Fatal("wrong timesheet identity")
	}
	w = call(v.service, "GET", "/v2/time/day?date=2026-11-02&person_id=EMP-0001", "", emp, false)
	if w.Code != 403 {
		t.Fatalf("cross employee timesheet leaked: %d", w.Code)
	}
	w = call(v.service, "GET", "/v2/time/quality", "", emp, false)
	if w.Code != 403 {
		t.Fatalf("employee got device quality: %d", w.Code)
	}
	w = call(v.service, "GET", "/v2/time/quality", "", hr, false)
	if w.Code != 200 {
		t.Fatalf("HR read quality failed: %d %s", w.Code, w.Body)
	}
	quality := obj(t, w.Body.Bytes())
	if int(quality["roster_rows"].(float64)) != 6000 {
		t.Fatalf("expected 6000 shifts, %v", quality)
	}
	if int(quality["raw_events"].(float64)) < 11000 {
		t.Fatalf("expected 12k simulated punches, %v", quality)
	}
	w = call(v.service, "GET", "/v2/time/pay-preview?period=2026-11", "", emp, false)
	if w.Code != 200 || obj(t, w.Body.Bytes())["blocked"] != true {
		t.Fatalf("preview not safely blocked: %d %s", w.Code, w.Body)
	}
	w = call(v.service, "GET", "/v2/time/pay-preview?person_id=EMP-0012&period=2026-11", "", admin, false)
	if w.Code != 200 {
		t.Fatalf("payroll admin preview: %d %s", w.Code, w.Body)
	}
}
func TestTimeIngestionIdempotencyAndQuarantine(t *testing.T) {
	v := secured(t)
	if err := attendanceSchema(v.service.db); err != nil {
		t.Fatal(err)
	}
	if err := seedAttendanceDemo(v.service.db); err != nil {
		t.Fatal(err)
	}
	hr := signin(t, v, "hr")
	emp := signin(t, v, "employee")
	b := map[string]any{"source": "TEST-API", "device_id": "SIM-GATE-01", "events": []any{
		map[string]string{"event_id": "event-001", "badge_id": "EMP-0012", "direction": "IN", "occurred_at": "2026-11-08T08:00:00+07:00"},
	}}
	content, _ := json.Marshal(b)
	if x := call(v.service, "POST", "/v2/time/punches/import", string(content), hr, false); x.Code != 403 {
		t.Fatalf("missing CSRF accepted: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/time/punches/import", string(content), emp, true); x.Code != 403 {
		t.Fatalf("employee device ingest accepted: %d", x.Code)
	}
	x := call(v.service, "POST", "/v2/time/punches/import", string(content), hr, true)
	if x.Code != 201 || int(obj(t, x.Body.Bytes())["accepted"].(float64)) != 1 {
		t.Fatalf("first import: %d %s", x.Code, x.Body)
	}
	x = call(v.service, "POST", "/v2/time/punches/import", string(content), hr, true)
	if x.Code != 201 || int(obj(t, x.Body.Bytes())["duplicate"].(float64)) != 1 {
		t.Fatalf("replay not idempotent: %d %s", x.Code, x.Body)
	}
	b["events"] = []any{map[string]string{"event_id": "event-001", "badge_id": "EMP-0012", "direction": "OUT", "occurred_at": "2026-11-08T08:00:00+07:00"}}
	content, _ = json.Marshal(b)
	x = call(v.service, "POST", "/v2/time/punches/import", string(content), hr, true)
	if x.Code != 409 {
		t.Fatalf("conflicting event accepted: %d %s", x.Code, x.Body)
	}
	b["events"] = []any{map[string]string{"event_id": "unknown-badge", "badge_id": "UNKNOWN-777", "direction": "IN", "occurred_at": "2026-11-08T08:00:00+07:00"}}
	content, _ = json.Marshal(b)
	x = call(v.service, "POST", "/v2/time/punches/import", string(content), hr, true)
	if x.Code != 201 || int(obj(t, x.Body.Bytes())["exceptions"].(float64)) != 1 {
		t.Fatalf("quarantine failed: %d %s", x.Code, x.Body)
	}
}
