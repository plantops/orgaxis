package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// P0 time-ledger review. This is NOT statutory payroll posting.
// A complete, externally sourced per-person day plan is mandatory before lock.
func backupBeforeTimeLedger(db SQLiteDB, dbPath string) (string, error) {
	existing, err := first(db, "SELECT name FROM sqlite_master WHERE type='table' AND name='oa_time_month_close'")
	if err != nil || existing != nil { return "", err }
	target := filepath.Join(filepath.Dir(dbPath), "orgaxis-before-time-ledger-"+time.Now().UTC().Format("20060102-150405.000000000")+".sqlite3")
	if err := db.Exec("VACUUM INTO ?", target); err != nil { return "", err }
	return target, nil
}

func timeLedgerSchema(db SQLiteDB) error {
	for _, sql := range []string{
		`CREATE TABLE IF NOT EXISTS oa_time_day_plan(
			tenant_id TEXT NOT NULL, person_id TEXT NOT NULL, work_date TEXT NOT NULL,
			kind TEXT NOT NULL CHECK(kind IN ('SHIFT','REST','HOLIDAY')),
			source_ref TEXT NOT NULL, edited_by TEXT NOT NULL, edited_at TEXT NOT NULL,
			PRIMARY KEY(tenant_id,person_id,work_date))`,
		`CREATE TABLE IF NOT EXISTS oa_time_month_close(
			tenant_id TEXT NOT NULL, person_id TEXT NOT NULL, period TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('PREPARED','LOCKED')),
			source_digest TEXT NOT NULL, shift_days INTEGER NOT NULL,
			scheduled_min INTEGER NOT NULL, regular_min INTEGER NOT NULL,
			ot_weekday_min INTEGER NOT NULL, ot_rest_min INTEGER NOT NULL, ot_holiday_min INTEGER NOT NULL,
			prepared_by TEXT NOT NULL, prepared_at TEXT NOT NULL,
			locked_by TEXT, locked_at TEXT,
			PRIMARY KEY(tenant_id,person_id,period))`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_close_immutable_update BEFORE UPDATE ON oa_time_month_close
			WHEN OLD.status='LOCKED' BEGIN SELECT RAISE(ABORT,'locked time ledger'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_close_immutable_delete BEFORE DELETE ON oa_time_month_close
			BEGIN SELECT RAISE(ABORT,'time ledger history cannot be deleted'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_plan_lock_insert BEFORE INSERT ON oa_time_day_plan
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=NEW.tenant_id AND c.person_id=NEW.person_id
				AND c.period=substr(NEW.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time day plan locked'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_plan_lock_update BEFORE UPDATE ON oa_time_day_plan
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=OLD.tenant_id AND c.person_id=OLD.person_id
				AND c.period=substr(OLD.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time day plan locked'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_plan_lock_delete BEFORE DELETE ON oa_time_day_plan
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=OLD.tenant_id AND c.person_id=OLD.person_id
				AND c.period=substr(OLD.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time day plan locked'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_roster_lock_insert BEFORE INSERT ON oa_time_roster
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=NEW.tenant_id AND c.person_id=NEW.person_id
				AND c.period=substr(NEW.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time roster locked'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_roster_lock_update BEFORE UPDATE ON oa_time_roster
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=OLD.tenant_id AND c.person_id=OLD.person_id
				AND c.period=substr(OLD.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time roster locked'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_time_roster_lock_delete BEFORE DELETE ON oa_time_roster
			WHEN EXISTS(SELECT 1 FROM oa_time_month_close c
				WHERE c.tenant_id=OLD.tenant_id AND c.person_id=OLD.person_id
				AND c.period=substr(OLD.work_date,1,7) AND c.status='LOCKED')
			BEGIN SELECT RAISE(ABORT,'time roster locked'); END`,
	} {
		if err := db.Exec(sql); err != nil { return err }
	}
	return nil
}

func monthWindow(period string) (time.Time, time.Time, error) {
	if len(period) != 7 || period[4] != '-' { return time.Time{}, time.Time{}, fail(422, "Period must be YYYY-MM") }
	d, err := time.Parse("2006-01", period)
	if err != nil || d.Format("2006-01") != period { return time.Time{}, time.Time{}, fail(422, "Invalid period") }
	return d, d.AddDate(0, 1, 0), nil
}
func dateOf(d time.Time) string { return d.Format("2006-01-02") }

type planDay struct {
	Date string `json:"date"`
	Kind string `json:"kind"`
}
type planBatch struct {
	PersonID string `json:"person_id"`
	Period string `json:"period"`
	SourceRef string `json:"source_ref"`
	Days []planDay `json:"days"`
}

// Plans may be replaced before the time ledger is locked. The source_ref is
// an evidence pointer, not a verification or proof of an authoritative ERP.
func (s *Service) importDayPlan(v planBatch, actor string) (any, error) {
	start, end, err := monthWindow(v.Period)
	if err != nil { return nil, err }
	if err := s.requirePerson(v.PersonID); err != nil { return nil, err }
	if strings.TrimSpace(v.SourceRef) == "" || len(v.SourceRef) > 160 || strings.ContainsAny(v.SourceRef, "\r\n") {
		return nil, fail(422, "Source reference is required (max 160 chars)")
	}
	days := int(end.Sub(start).Hours() / 24)
	if len(v.Days) != days { return nil, fail(422, "Explicit plan must include every calendar date") }
	seen := map[string]bool{}
	for _, day := range v.Days {
		if seen[day.Date] || !strings.HasPrefix(day.Date, v.Period+"-") ||
			(day.Kind != "SHIFT" && day.Kind != "REST" && day.Kind != "HOLIDAY") {
			return nil, fail(422, "Invalid or duplicated day plan entry")
		}
		at, e := time.Parse("2006-01-02", day.Date)
		if e != nil || at.Before(start) || !at.Before(end) || dateOf(at) != day.Date {
			return nil, fail(422, "Day plan contains invalid date")
		}
		seen[day.Date] = true
	}
	if err = s.db.Exec("BEGIN IMMEDIATE"); err != nil { return nil, err }
	committed := false
	defer func() { if !committed { _ = s.db.Exec("ROLLBACK") } }()
	closed, err := s.one(`SELECT status FROM oa_time_month_close
		WHERE tenant_id=? AND person_id=? AND period=?`, tenantID, v.PersonID, v.Period)
	if err != nil { return nil, err }
	if rowString(closed, "status") == "LOCKED" { return nil, fail(409, "Time ledger locked; use correction workflow") }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, day := range v.Days {
		if err = s.db.Exec(`INSERT INTO oa_time_day_plan
			(tenant_id,person_id,work_date,kind,source_ref,edited_by,edited_at)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(tenant_id,person_id,work_date) DO UPDATE SET
			kind=excluded.kind,source_ref=excluded.source_ref,
			edited_by=excluded.edited_by,edited_at=excluded.edited_at`,
			tenantID, v.PersonID, day.Date, day.Kind, v.SourceRef, actor, now); err != nil { return nil, err }
	}
	if err = auditV6(s, actor, "TIME_DAY_PLAN_IMPORT", v.PersonID,
		map[string]any{"period":v.Period,"days":len(v.Days),"source_ref":v.SourceRef}); err != nil { return nil, err }
	if err = s.db.Exec("COMMIT"); err != nil { return nil, err }
	committed = true
	return map[string]any{"person_id":v.PersonID,"period":v.Period,"days":days,"source_ref":v.SourceRef}, nil
}

type periodReadiness struct {
	PersonID string `json:"person_id"`
	Period string `json:"period"`
	Ready bool `json:"ready"`
	Blockers []string `json:"blockers"`
	ShiftDays int `json:"shift_days"`
	ScheduledMinutes int `json:"scheduled_minutes"`
	RegularMinutes int `json:"regular_minutes"`
	OTWeekdayMinutes int `json:"ot_weekday_minutes"`
	OTRestMinutes int `json:"ot_rest_minutes"`
	OTHolidayMinutes int `json:"ot_holiday_minutes"`
	SourceDigest string `json:"source_digest"`
}

func (s *Service) periodDigest(person, period string, start, end time.Time) (string, error) {
	// Include source rows as well as time valuations. Explicit ORDER BY makes
	// the snapshot repeatable across process restart and SQLite query plans.
	// The observation range admits shift punches on either side of midnight.
	tz, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil { return "", err }
	localStart := time.Date(start.Year(),start.Month(),1,0,0,0,0,tz)
	localEnd := localStart.AddDate(0,1,0)
	lo := localStart.Add(-4*time.Hour).UTC().Format(time.RFC3339Nano)
	hi := localEnd.Add(6*time.Hour).UTC().Format(time.RFC3339Nano)
	tables := []struct { name, sql string; args []any }{
		{"plan", `SELECT work_date,kind,source_ref,edited_by,edited_at FROM oa_time_day_plan WHERE tenant_id=? AND person_id=? AND work_date LIKE ? ORDER BY work_date`, []any{tenantID,person,period+"%"}},
		{"roster", `SELECT work_date,shift_start,shift_end,unpaid_break_min,origin FROM oa_time_roster WHERE tenant_id=? AND person_id=? AND work_date LIKE ? ORDER BY work_date`, []any{tenantID,person,period+"%"}},
		{"raw", `SELECT source,device_id,event_id,badge_id,direction,occurred_at_utc,received_at_utc FROM oa_raw_punch WHERE tenant_id=? AND person_id=? AND occurred_at_utc>=? AND occurred_at_utc<? ORDER BY occurred_at_utc,source,device_id,event_id`, []any{tenantID,person,lo,hi}},
		{"leave", `SELECT approval_id,leave_start,leave_end,paid,approved FROM oa_time_leave_credit WHERE tenant_id=? AND person_id=? AND leave_start<? AND leave_end>? ORDER BY approval_id`, []any{tenantID,person,hi,lo}},
		{"ot", `SELECT approval_id,ot_start,ot_end,kind,approved FROM oa_time_ot WHERE tenant_id=? AND person_id=? AND ot_start<? AND ot_end>? ORDER BY approval_id`, []any{tenantID,person,hi,lo}},
	}
	h := sha256.New()
	for _, spec := range tables {
		rows, err := s.rows(spec.sql,spec.args...)
		if err != nil { return "", err }
		blob, err := json.Marshal(rows)
		if err != nil { return "", err }
		_, _ = h.Write([]byte(spec.name + ":"))
		_, _ = h.Write(blob)
		_, _ = h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) timePeriodReadiness(person, period string) (periodReadiness, error) {
	start, end, err := monthWindow(period)
	if err != nil { return periodReadiness{}, err }
	if err := s.requirePerson(person); err != nil { return periodReadiness{}, err }
	out := periodReadiness{PersonID:person,Period:period,Blockers:[]string{}}
	type observedWindow struct { start, end time.Time }
	windows := []observedWindow{}
	plans, err := s.rows(`SELECT work_date,kind,source_ref FROM oa_time_day_plan
		WHERE tenant_id=? AND person_id=? AND work_date LIKE ? ORDER BY work_date`,tenantID,person,period+"%")
	if err != nil { return out, err }
	byDate := map[string]Row{}
	for _, p := range plans { byDate[rowString(p,"work_date")] = p }
	for d := start; d.Before(end); d = d.AddDate(0,0,1) {
		day := dateOf(d)
		p := byDate[day]
		if p == nil {
			out.Blockers = append(out.Blockers, day+":MISSING_VERIFIED_DAY_PLAN")
			continue
		}
		if strings.HasPrefix(strings.ToUpper(rowString(p,"source_ref")),"SYNTHETIC") {
			out.Blockers = append(out.Blockers, day+":SYNTHETIC_PLAN_NOT_ACCEPTABLE")
		}
		kind := rowString(p,"kind")
		roster, e := s.one(`SELECT origin,shift_start,shift_end FROM oa_time_roster WHERE tenant_id=? AND person_id=? AND work_date=?`, tenantID,person,day)
		if e != nil { return out, e }
		if kind != "SHIFT" {
			if roster != nil { out.Blockers = append(out.Blockers,day+":NON_SHIFT_HAS_ROSTER") }
			continue
		}
		out.ShiftDays++
		if roster == nil {
			out.Blockers = append(out.Blockers,day+":MISSING_SHIFT_ROSTER")
			continue
		}
		if strings.HasPrefix(strings.ToUpper(rowString(roster,"origin")),"SYNTHETIC") {
			out.Blockers = append(out.Blockers,day+":SYNTHETIC_ROSTER_NOT_ACCEPTABLE")
		}
		startAt,startErr:=parseAt(rowString(roster,"shift_start"))
		endAt,endErr:=parseAt(rowString(roster,"shift_end"))
		if startErr!=nil || endErr!=nil { return out,fail(422,"Invalid roster timestamps") }
		windows=append(windows,observedWindow{startAt.Add(-4*time.Hour),endAt.Add(6*time.Hour)})
		valuation,e := s.attendanceDay(person,day)
		if e != nil { return out,e }
		out.ScheduledMinutes += valuation.ScheduledMinutes
		out.RegularMinutes += valuation.CreditedRegularMinutes
		out.OTWeekdayMinutes += valuation.ApprovedOTMinutes["WEEKDAY"]
		out.OTRestMinutes += valuation.ApprovedOTMinutes["REST_DAY"]
		out.OTHolidayMinutes += valuation.ApprovedOTMinutes["HOLIDAY"]
		if !valuation.CanClose {
			out.Blockers = append(out.Blockers,day+":"+strings.Join(valuation.Exceptions,","))
		}
	}
	if out.ShiftDays == 0 { out.Blockers = append(out.Blockers,"NO_SHIFT_DAYS") }
	// A shift starting on the last day of the previous period may carry over
	// into the first calendar day. Include its observation window.
	prior,e:=s.one(`SELECT shift_start,shift_end FROM oa_time_roster
		WHERE tenant_id=? AND person_id=? AND work_date=?`,tenantID,person,dateOf(start.AddDate(0,0,-1)))
	if e!=nil{return out,e}
	if prior!=nil{
		a,ea:=parseAt(rowString(prior,"shift_start"))
		b,eb:=parseAt(rowString(prior,"shift_end"))
		if ea!=nil || eb!=nil{return out,fail(422,"Invalid prior shift timestamps")}
		windows=append(windows,observedWindow{a.Add(-4*time.Hour),b.Add(6*time.Hour)})
	}
	siteTZ,e:=time.LoadLocation("Asia/Ho_Chi_Minh")
	if e!=nil{return out,e}
	siteStart:=time.Date(start.Year(),start.Month(),1,0,0,0,0,siteTZ)
	siteEnd:=siteStart.AddDate(0,1,0)
	orphans,e:=s.rows(`SELECT event_id,occurred_at_utc FROM oa_raw_punch
		WHERE tenant_id=? AND person_id=? AND occurred_at_utc>=? AND occurred_at_utc<?
		ORDER BY occurred_at_utc,event_id`,tenantID,person,
		siteStart.UTC().Format(time.RFC3339Nano),siteEnd.UTC().Format(time.RFC3339Nano))
	if e!=nil{return out,e}
	for _,punch:=range orphans{
		at,err:=parseAt(rowString(punch,"occurred_at_utc"))
		if err!=nil{return out,err}
		covered:=false
		for _,w:=range windows{
			if !at.Before(w.start) && !at.After(w.end){covered=true;break}
		}
		if !covered{out.Blockers=append(out.Blockers,"UNROSTERED_PUNCH:"+rowString(punch,"event_id"))}
	}
	// The current demo OT/leave credit rows are not linked to approved
	// workflow evidence. Block period close until that integration exists.
	lo := start.Add(-2*24*time.Hour).Format(time.RFC3339)
	hi := end.Add(2*24*time.Hour).Format(time.RFC3339)
	credit,e := s.one(`SELECT
		(SELECT COUNT(*) FROM oa_time_ot WHERE tenant_id=? AND person_id=? AND ot_start<? AND ot_end>?)+
		(SELECT COUNT(*) FROM oa_time_leave_credit WHERE tenant_id=? AND person_id=? AND leave_start<? AND leave_end>?) AS n`,
		tenantID,person,hi,lo,tenantID,person,hi,lo)
	if e != nil { return out,e }
	if intval(credit["n"])>0 { out.Blockers=append(out.Blockers,"LEAVE_OT_APPROVAL_PROVENANCE_UNVERIFIED") }
	// Quarantined unknown badges cannot yet be safely assigned to a person.
	// Hold all monthly locks in this tenant until the source exceptions are reviewed.
	bad,e := s.one(`SELECT COUNT(*) AS n FROM oa_time_ingest_exception
		WHERE tenant_id=? AND status='PENDING' AND occurred_at>=? AND occurred_at<?`, tenantID,lo,hi)
	if e != nil { return out,e }
	if intval(bad["n"])>0 { out.Blockers=append(out.Blockers,"PENDING_DEVICE_QUARANTINE") }
	digest,err := s.periodDigest(person,period,start,end)
	if err != nil { return out,err }
	out.SourceDigest=digest
	out.Ready=len(out.Blockers)==0
	return out,nil
}

func (s *Service) prepareTimePeriod(person,period,actor string) (any,error) {
	r,err := s.timePeriodReadiness(person,period)
	if err != nil {return nil,err}
	if !r.Ready { return nil,fail(409,"Time period blocked: "+strings.Join(r.Blockers,"; ")) }
	if err=s.db.Exec("BEGIN IMMEDIATE");err!=nil{return nil,err}
	committed:=false
	defer func(){if !committed{_ = s.db.Exec("ROLLBACK")}}()
	old,err:=s.one(`SELECT status FROM oa_time_month_close WHERE tenant_id=? AND person_id=? AND period=?`,tenantID,person,period)
	if err!=nil{return nil,err}
	if rowString(old,"status")=="LOCKED"{return nil,fail(409,"Time ledger already locked")}
	if err=s.db.Exec(`INSERT INTO oa_time_month_close
		(tenant_id,person_id,period,status,source_digest,shift_days,scheduled_min,regular_min,ot_weekday_min,ot_rest_min,ot_holiday_min,prepared_by,prepared_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(tenant_id,person_id,period) DO UPDATE SET
		status=excluded.status,source_digest=excluded.source_digest,shift_days=excluded.shift_days,
		scheduled_min=excluded.scheduled_min,regular_min=excluded.regular_min,
		ot_weekday_min=excluded.ot_weekday_min,ot_rest_min=excluded.ot_rest_min,ot_holiday_min=excluded.ot_holiday_min,
		prepared_by=excluded.prepared_by,prepared_at=excluded.prepared_at,
		locked_by=NULL,locked_at=NULL`,
		tenantID,person,period,"PREPARED",r.SourceDigest,r.ShiftDays,r.ScheduledMinutes,r.RegularMinutes,
		r.OTWeekdayMinutes,r.OTRestMinutes,r.OTHolidayMinutes,actor,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return nil,err}
	if err=auditV6(s,actor,"TIME_PERIOD_PREPARE",person,map[string]any{"period":period,"digest":r.SourceDigest});err!=nil{return nil,err}
	if err=s.db.Exec("COMMIT");err!=nil{return nil,err}
	committed=true
	return map[string]any{"status":"PREPARED","readiness":r},nil
}

func (s *Service) approveTimePeriod(person,period,actor string) (any,error) {
	r,err:=s.timePeriodReadiness(person,period)
	if err!=nil{return nil,err}
	if !r.Ready{return nil,fail(409,"Time period blocked: "+strings.Join(r.Blockers,"; "))}
	if err=s.db.Exec("BEGIN IMMEDIATE");err!=nil{return nil,err}
	committed:=false
	defer func(){if !committed{_ = s.db.Exec("ROLLBACK")}}()
	prepared,err:=s.one(`SELECT status,source_digest,prepared_by FROM oa_time_month_close
		WHERE tenant_id=? AND person_id=? AND period=?`,tenantID,person,period)
	if err!=nil{return nil,err}
	if prepared==nil || rowString(prepared,"status")!="PREPARED"{return nil,fail(409,"No pending time ledger review")}
	if rowString(prepared,"prepared_by")==actor{return nil,fail(403,"Independent payroll reviewer required")}
	if rowString(prepared,"source_digest")!=r.SourceDigest{return nil,fail(409,"Time sources changed since HR preparation")}
	if err=s.db.Exec(`UPDATE oa_time_month_close SET status='LOCKED',locked_by=?,locked_at=?
		WHERE tenant_id=? AND person_id=? AND period=? AND status='PREPARED'`,
		actor,time.Now().UTC().Format(time.RFC3339Nano),tenantID,person,period);err!=nil{return nil,err}
	if err=auditV6(s,actor,"TIME_PERIOD_LOCK",person,map[string]any{"period":period,"digest":r.SourceDigest});err!=nil{return nil,err}
	if err=s.db.Exec("COMMIT");err!=nil{return nil,err}
	committed=true
	return map[string]any{"status":"LOCKED","period":period,"person_id":person,"source_digest":r.SourceDigest,
		"note":"Time ledger snapshot only; no payroll posted or legal validation implied"},nil
}

func (s *Service) verifyTimePeriod(person,period string) (any,error) {
	_,_,err:=monthWindow(period)
	if err!=nil{return nil,err}
	row,err:=s.one(`SELECT status,source_digest,prepared_by,prepared_at,locked_by,locked_at,
		shift_days,scheduled_min,regular_min,ot_weekday_min,ot_rest_min,ot_holiday_min
		FROM oa_time_month_close WHERE tenant_id=? AND person_id=? AND period=?`,tenantID,person,period)
	if err!=nil{return nil,err}
	if row==nil{return nil,fail(404,"No prepared time ledger")}
	start,end,_:=monthWindow(period)
	live,err:=s.periodDigest(person,period,start,end)
	if err!=nil{return nil,err}
	return map[string]any{"snapshot":row,"person_id":person,"period":period,
		"source_unchanged":live==rowString(row,"source_digest"),
		"current_source_digest":live,"posted_payroll":false},nil
}

func (s *Service) timeCloseAPI(p *Principal,r *http.Request) (any,error) {
	path:=r.URL.Path
	if !role(p,"HR","PAYROLL","ADMIN"){return nil,fail(403,"Time ledger role required")}
	if path=="/v2/time/calendar/import" && r.Method=="POST" {
		if !role(p,"HR","ADMIN"){return nil,fail(403,"HR plan import required")}
		var v planBatch
		if err:=bodyJSON(r,&v);err!=nil{return nil,err}
		return s.importDayPlan(v,p.PersonID)
	}
	if r.Method=="GET" && (path=="/v2/time/period/readiness" || path=="/v2/time/period/verify") {
		person,period:=r.URL.Query().Get("person_id"),r.URL.Query().Get("period")
		if person==""{person=p.PersonID}
		if path=="/v2/time/period/readiness"{return s.timePeriodReadiness(person,period)}
		return s.verifyTimePeriod(person,period)
	}
	if r.Method=="POST" && (path=="/v2/time/period/prepare" || path=="/v2/time/period/approve") {
		var v struct {PersonID string `json:"person_id"`; Period string `json:"period"`}
		if err:=bodyJSON(r,&v);err!=nil{return nil,err}
		if path=="/v2/time/period/prepare" {
			if !role(p,"HR","ADMIN"){return nil,fail(403,"HR time preparation required")}
			return s.prepareTimePeriod(v.PersonID,v.Period,p.PersonID)
		}
		if !role(p,"PAYROLL"){return nil,fail(403,"Independent payroll reviewer required")}
		return s.approveTimePeriod(v.PersonID,v.Period,p.PersonID)
	}
	return nil,fail(404,"Unknown time ledger endpoint")
}

