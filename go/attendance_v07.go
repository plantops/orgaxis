package main

import (
	"fmt"
	"github.com/plantops/orgaxis/timecore"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// Attendance v0.7 is a synthetic-data integration contract. It does NOT
// connect to actual biometric devices or calculate statutory take-home pay.
// Raw device events are retained with source provenance and stable event keys.
func attendanceSchema(db SQLiteDB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS oa_time_device(tenant_id TEXT NOT NULL,device_id TEXT NOT NULL,site_id TEXT NOT NULL,description TEXT NOT NULL,source_type TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 1,PRIMARY KEY(tenant_id,device_id))`,
		`CREATE TABLE IF NOT EXISTS oa_badge_binding(tenant_id TEXT NOT NULL,device_id TEXT NOT NULL,badge_id TEXT NOT NULL,person_id TEXT NOT NULL,valid_from TEXT NOT NULL,valid_to TEXT,PRIMARY KEY(tenant_id,device_id,badge_id,valid_from))`,
		`CREATE TABLE IF NOT EXISTS oa_raw_punch(tenant_id TEXT NOT NULL,source TEXT NOT NULL,device_id TEXT NOT NULL,event_id TEXT NOT NULL,badge_id TEXT NOT NULL,person_id TEXT NOT NULL,direction TEXT NOT NULL,occurred_at_utc TEXT NOT NULL,received_at_utc TEXT NOT NULL,PRIMARY KEY(tenant_id,source,device_id,event_id))`,
		`CREATE INDEX IF NOT EXISTS ix_oa_punch_person_at ON oa_raw_punch(tenant_id,person_id,occurred_at_utc)`,
		`CREATE TABLE IF NOT EXISTS oa_time_roster(tenant_id TEXT NOT NULL,person_id TEXT NOT NULL,work_date TEXT NOT NULL,shift_start TEXT NOT NULL,shift_end TEXT NOT NULL,unpaid_break_min INTEGER NOT NULL CHECK(unpaid_break_min>=0),origin TEXT NOT NULL,PRIMARY KEY(tenant_id,person_id,work_date))`,
		`CREATE INDEX IF NOT EXISTS ix_oa_roster_date ON oa_time_roster(tenant_id,work_date)`,
		`CREATE TABLE IF NOT EXISTS oa_time_ot(tenant_id TEXT NOT NULL,person_id TEXT NOT NULL,approval_id TEXT NOT NULL,ot_start TEXT NOT NULL,ot_end TEXT NOT NULL,kind TEXT NOT NULL,approved INTEGER NOT NULL CHECK(approved IN (0,1)),PRIMARY KEY(tenant_id,approval_id))`,
		`CREATE TABLE IF NOT EXISTS oa_time_leave_credit(tenant_id TEXT NOT NULL,person_id TEXT NOT NULL,approval_id TEXT NOT NULL,leave_start TEXT NOT NULL,leave_end TEXT NOT NULL,paid INTEGER NOT NULL CHECK(paid IN (0,1)),approved INTEGER NOT NULL CHECK(approved IN (0,1)),PRIMARY KEY(tenant_id,approval_id))`,
		`CREATE TABLE IF NOT EXISTS oa_time_ingest_exception(id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,source TEXT NOT NULL,device_id TEXT NOT NULL,event_id TEXT NOT NULL,badge_id TEXT NOT NULL,direction TEXT NOT NULL,reason TEXT NOT NULL,occurred_at TEXT NOT NULL,received_at TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'PENDING',UNIQUE(tenant_id,source,device_id,event_id))`,
	}
	for _, q := range statements {
		if e := db.Exec(q); e != nil {
			return e
		}
	}
	return nil
}

type DeviceBatch struct {
	Source   string `json:"source"`
	DeviceID string `json:"device_id"`
	Events   []struct {
		EventID    string `json:"event_id"`
		BadgeID    string `json:"badge_id"`
		Direction  string `json:"direction"`
		OccurredAt string `json:"occurred_at"` // RFC3339 offset required
	} `json:"events"`
}

var sourceIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,80}$`)

func (s *Service) ingestPunches(batch DeviceBatch, actor string) (any, error) {
	if !sourceIdentifier.MatchString(batch.Source) || !sourceIdentifier.MatchString(batch.DeviceID) || len(batch.Events) == 0 || len(batch.Events) > 2000 {
		return nil, fail(422, "Invalid source, device or batch size (1..2000)")
	}
	dev, e := s.one(`SELECT enabled FROM oa_time_device WHERE tenant_id=? AND device_id=?`, tenantID, batch.DeviceID)
	if e != nil {
		return nil, e
	}
	if dev == nil || intval(dev["enabled"]) != 1 {
		return nil, fail(422, "Unknown/disabled time device")
	}
	if e = s.db.Exec("BEGIN IMMEDIATE"); e != nil {
		return nil, e
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.db.Exec("ROLLBACK")
		}
	}()
	accepted, duplicated, exceptions := 0, 0, 0
	for _, ev := range batch.Events {
		if !sourceIdentifier.MatchString(ev.EventID) || !sourceIdentifier.MatchString(ev.BadgeID) || len(ev.OccurredAt) > 64 {
			return nil, fail(422, "Invalid external event")
		}
		t, err := time.Parse(time.RFC3339Nano, ev.OccurredAt)
		if err != nil {
			return nil, fail(422, "Timestamp must be RFC3339 with timezone")
		}
		atUTC := t.UTC().Format(time.RFC3339Nano)
		old, err := s.one(`SELECT badge_id,direction,occurred_at_utc FROM oa_raw_punch WHERE tenant_id=? AND source=? AND device_id=? AND event_id=?`, tenantID, batch.Source, batch.DeviceID, ev.EventID)
		if err != nil {
			return nil, err
		}
		if old != nil {
			if rowString(old, "badge_id") != ev.BadgeID || rowString(old, "direction") != ev.Direction || rowString(old, "occurred_at_utc") != atUTC {
				return nil, fail(409, "Conflicting event ID; batch rolled back")
			}
			duplicated++
			continue
		}
		mapped, err := s.one(`SELECT person_id FROM oa_badge_binding WHERE tenant_id=? AND device_id=? AND badge_id=? AND valid_from<=? AND (valid_to IS NULL OR valid_to>?) ORDER BY valid_from DESC LIMIT 1`, tenantID, batch.DeviceID, ev.BadgeID, t.Format("2006-01-02"), t.Format("2006-01-02"))
		if err != nil {
			return nil, err
		}
		reason := ""
		if mapped == nil {
			reason = "UNKNOWN_BADGE"
		} else if ev.Direction != timecore.DirectionIn && ev.Direction != timecore.DirectionOut {
			reason = "UNKNOWN_DIRECTION"
		}
		if reason != "" {
			prior, pe := s.one(`SELECT badge_id,direction,occurred_at FROM oa_time_ingest_exception WHERE tenant_id=? AND source=? AND device_id=? AND event_id=?`, tenantID, batch.Source, batch.DeviceID, ev.EventID)
			if pe != nil {
				return nil, pe
			}
			if prior != nil {
				if rowString(prior, "badge_id") != ev.BadgeID || rowString(prior, "direction") != ev.Direction || rowString(prior, "occurred_at") != atUTC {
					return nil, fail(409, "Conflicting quarantined event ID; batch rolled back")
				}
				duplicated++
			} else {
				if err = s.db.Exec(`INSERT INTO oa_time_ingest_exception(id,tenant_id,source,device_id,event_id,badge_id,direction,reason,occurred_at,received_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, uid("EXC"), tenantID, batch.Source, batch.DeviceID, ev.EventID, ev.BadgeID, ev.Direction, reason, atUTC, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					return nil, err
				}
				exceptions++
			}
			continue
		}
		// Reprocessing a previously quarantined event after a badge mapping
		// was corrected is allowed; retain the original exception as RESOLVED.
		prior, pe := s.one(`SELECT badge_id,direction,occurred_at FROM oa_time_ingest_exception WHERE tenant_id=? AND source=? AND device_id=? AND event_id=?`, tenantID, batch.Source, batch.DeviceID, ev.EventID)
		if pe != nil {
			return nil, pe
		}
		if prior != nil && (rowString(prior, "badge_id") != ev.BadgeID || rowString(prior, "direction") != ev.Direction || rowString(prior, "occurred_at") != atUTC) {
			return nil, fail(409, "Conflicting quarantined event ID; batch rolled back")
		}

		if err = s.db.Exec(`INSERT INTO oa_raw_punch VALUES(?,?,?,?,?,?,?,?,?)`, tenantID, batch.Source, batch.DeviceID, ev.EventID, ev.BadgeID, rowString(mapped, "person_id"), ev.Direction, atUTC, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return nil, err
		}
		if err = s.db.Exec(`UPDATE oa_time_ingest_exception SET status='RESOLVED' WHERE tenant_id=? AND source=? AND device_id=? AND event_id=?`, tenantID, batch.Source, batch.DeviceID, ev.EventID); err != nil {
			return nil, err
		}
		accepted++
	}
	if e = s.db.Exec("COMMIT"); e != nil {
		return nil, e
	}
	committed = true
	_ = auditV6(s, actor, "TIME_PUNCH_IMPORT", batch.DeviceID, map[string]any{"accepted": accepted, "duplicate": duplicated, "exceptions": exceptions})
	return map[string]any{"accepted": accepted, "duplicate": duplicated, "exceptions": exceptions, "source": batch.Source, "device_id": batch.DeviceID}, nil
}
func parseAt(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
func (s *Service) attendanceDay(person, date string) (timecore.Daily, error) {
	if _, e := time.Parse("2006-01-02", date); e != nil {
		return timecore.Daily{}, fail(422, "Invalid work date")
	}
	r, e := s.one(`SELECT * FROM oa_time_roster WHERE tenant_id=? AND person_id=? AND work_date=?`, tenantID, person, date)
	if e != nil {
		return timecore.Daily{}, e
	}
	if r == nil {
		return timecore.Daily{}, fail(404, "No roster for person/date")
	}
	start, e := parseAt(rowString(r, "shift_start"))
	if e != nil {
		return timecore.Daily{}, e
	}
	end, e := parseAt(rowString(r, "shift_end"))
	if e != nil {
		return timecore.Daily{}, e
	}
	roster := timecore.Shift{PersonID: person, WorkDate: date, Start: start, End: end, UnpaidBreakMinutes: intval(r["unpaid_break_min"])}
	low := start.Add(-4 * time.Hour).UTC().Format(time.RFC3339Nano)
	high := end.Add(6 * time.Hour).UTC().Format(time.RFC3339Nano)
	raw, e := s.rows(`SELECT source,device_id,event_id,person_id,direction,occurred_at_utc FROM oa_raw_punch WHERE tenant_id=? AND person_id=? AND occurred_at_utc>=? AND occurred_at_utc<=? ORDER BY occurred_at_utc`, tenantID, person, low, high)
	if e != nil {
		return timecore.Daily{}, e
	}
	punches := make([]timecore.Punch, 0, len(raw))
	for _, r := range raw {
		tm, e := parseAt(rowString(r, "occurred_at_utc"))
		if e != nil {
			return timecore.Daily{}, e
		}
		punches = append(punches, timecore.Punch{Source: rowString(r, "source"), DeviceID: rowString(r, "device_id"), EventID: rowString(r, "event_id"), PersonID: person, Direction: rowString(r, "direction"), At: tm})
	}
	lv, e := s.rows(`SELECT leave_start,leave_end,paid,approved FROM oa_time_leave_credit WHERE tenant_id=? AND person_id=? AND leave_start<? AND leave_end>?`, tenantID, person, end.Format(time.RFC3339), start.Format(time.RFC3339))
	if e != nil {
		return timecore.Daily{}, e
	}
	leaves := make([]timecore.Leave, 0, len(lv))
	for _, r := range lv {
		a, e1 := parseAt(rowString(r, "leave_start"))
		b, e2 := parseAt(rowString(r, "leave_end"))
		if e1 != nil || e2 != nil {
			return timecore.Daily{}, fmt.Errorf("bad leave credit")
		}
		leaves = append(leaves, timecore.Leave{Start: a, End: b, Paid: intval(r["paid"]) != 0, Approved: intval(r["approved"]) != 0})
	}
	otRows, e := s.rows(`SELECT ot_start,ot_end,kind,approved FROM oa_time_ot WHERE tenant_id=? AND person_id=? AND ot_start<? AND ot_end>?`, tenantID, person, end.Add(6*time.Hour).Format(time.RFC3339), start.Add(-4*time.Hour).Format(time.RFC3339))
	if e != nil {
		return timecore.Daily{}, e
	}
	ots := make([]timecore.OTApproval, 0, len(otRows))
	for _, r := range otRows {
		a, e1 := parseAt(rowString(r, "ot_start"))
		b, e2 := parseAt(rowString(r, "ot_end"))
		if e1 != nil || e2 != nil {
			return timecore.Daily{}, fmt.Errorf("bad OT credit")
		}
		ots = append(ots, timecore.OTApproval{Start: a, End: b, Kind: rowString(r, "kind"), Approved: intval(r["approved"]) != 0})
	}
	return timecore.Evaluate(roster, punches, leaves, ots)
}
func (s *Service) attendanceAPI(p *Principal, r *http.Request) (any, error) {
	path := r.URL.Path
	switch {
	case path == "/v2/time/devices" && r.Method == "GET":
		if !role(p, "HR", "ADMIN") {
			return nil, fail(403, "Time device administration required")
		}
		v, e := s.rows(`SELECT device_id,site_id,description,source_type,enabled FROM oa_time_device WHERE tenant_id=?`, tenantID)
		return map[string]any{"devices": v}, e
	case path == "/v2/time/punches/import" && r.Method == "POST":
		if !role(p, "HR", "ADMIN") {
			return nil, fail(403, "Time data ingestion permission required")
		}
		var b DeviceBatch
		if e := bodyJSON(r, &b); e != nil {
			return nil, e
		}
		return s.ingestPunches(b, p.PersonID)
	case path == "/v2/time/day" && r.Method == "GET":
		person := r.URL.Query().Get("person_id")
		if person == "" {
			person = p.PersonID
		}
		if !s.canSeePerson(p, person, time.Now().Format("2006-01-02")) {
			return nil, fail(403, "Time record out of scope")
		}
		return s.attendanceDay(person, r.URL.Query().Get("date"))
	case path == "/v2/time/quality" && r.Method == "GET":
		if !role(p, "HR", "ADMIN") {
			return nil, fail(403, "Time quality permission required")
		}
		raw, e := s.one(`SELECT COUNT(*) AS n FROM oa_raw_punch WHERE tenant_id=?`, tenantID)
		if e != nil {
			return nil, e
		}
		bad, e := s.rows(`SELECT reason,status,COUNT(*) AS count FROM oa_time_ingest_exception WHERE tenant_id=? GROUP BY reason,status`, tenantID)
		if e != nil {
			return nil, e
		}
		roster, e := s.one(`SELECT COUNT(*) AS n FROM oa_time_roster WHERE tenant_id=?`, tenantID)
		if e != nil {
			return nil, e
		}
		return map[string]any{"raw_events": intval(raw["n"]), "roster_rows": intval(roster["n"]), "ingest_exceptions": bad, "note": "Synthetic device integration, no actual hardware connected"}, nil
	case path == "/v2/time/pay-preview" && r.Method == "GET":
		person := r.URL.Query().Get("person_id")
		if person == "" {
			person = p.PersonID
		}
		if !s.canSeePayroll(p, person) {
			return nil, fail(403, "Payroll record out of scope")
		}
		period := r.URL.Query().Get("period")
		if len(period) != 7 || period[4] != '-' {
			return nil, fail(422, "Specify period YYYY-MM")
		}
		if _, e := time.Parse("2006-01", period); e != nil {
			return nil, fail(422, "Bad payroll period")
		}
		rws, e := s.rows(`SELECT work_date FROM oa_time_roster WHERE tenant_id=? AND person_id=? AND work_date LIKE ? ORDER BY work_date`, tenantID, person, period+"%")
		if e != nil {
			return nil, e
		}
		days := make([]timecore.Daily, 0, len(rws))
		for _, row := range rws {
			day, e := s.attendanceDay(person, rowString(row, "work_date"))
			if e != nil {
				return nil, e
			}
			days = append(days, day)
		}
		pay, e := s.one(`SELECT base_pay FROM payroll_demo_ledger WHERE tenant_id=? AND person_id=? AND period=?`, tenantID, person, period)
		if e != nil {
			return nil, e
		}
		if pay == nil {
			return nil, fail(404, "No payroll demo base for period")
		}
		pv, e := timecore.PreviewMonthlyPay(person, period, int64(intval(pay["base_pay"])), days)
		if e != nil {
			return nil, e
		}
		// Do not describe partial-period previews as posted payroll.
		expectedWorkdays := 0
		mm, _ := strconv.Atoi(period[5:])
		if mm >= 1 && mm <= 12 {
			start, _ := time.Parse("2006-01-02", period+"-01")
			for d := start; d.Month() == start.Month(); d = d.AddDate(0, 0, 1) {
				if d.Weekday() != time.Sunday {
					expectedWorkdays++
				}
			}
		}
		if len(days) < expectedWorkdays {
			pv.Blocked = true
			pv.Exceptions = append(pv.Exceptions, "INCOMPLETE_ROSTER_PERIOD")
		}
		return pv, nil
	}
	return nil, fail(404, "Unknown time endpoint")
}
