//go:build linux && cgo

package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestV09SyntheticMonthCannotClose(t *testing.T) {
	v := secured(t)
	if e := attendanceSchema(v.service.db); e != nil { t.Fatal(e) }
	if e := attendanceCheckpointSchema(v.service.db); e != nil { t.Fatal(e) }
	if e := timeLedgerSchema(v.service.db); e != nil { t.Fatal(e) }
	if e := seedAttendanceDemo(v.service.db); e != nil { t.Fatal(e) }
	hr := signin(t,v,"hr")
	body := `{"person_id":"EMP-0012","period":"2026-11"}`
	x := call(v.service,"POST","/v2/time/period/prepare",body,hr,true)
	if x.Code != 409 { t.Fatalf("synthetic incomplete period close accepted: %d %s",x.Code,x.Body) }
	x = call(v.service,"GET","/v2/time/period/readiness?person_id=EMP-0012&period=2026-11","",hr,false)
	if x.Code != 200 || obj(t,x.Body.Bytes())["ready"] != false {
		t.Fatalf("incomplete period readiness: %d %s",x.Code,x.Body)
	}
	closed,e := v.service.one(`SELECT status FROM oa_time_month_close WHERE tenant_id=? AND person_id='EMP-0012' AND period='2026-11'`,tenantID)
	if e!=nil || closed!=nil {t.Fatalf("blocked period inserted snapshot %v %v",closed,e)}
}

func setupV09ReadyMonth(t *testing.T, v *secureTest) planBatch {
	t.Helper()
	if e:=attendanceSchema(v.service.db);e!=nil{t.Fatal(e)}
	if e:=attendanceCheckpointSchema(v.service.db);e!=nil{t.Fatal(e)}
	if e:=timeLedgerSchema(v.service.db);e!=nil{t.Fatal(e)}
	if e:=seedAttendanceDemo(v.service.db);e!=nil{t.Fatal(e)}
	loc:=time.FixedZone("VN",7*3600)
	plan:=planBatch{PersonID:"EMP-0012",Period:"2026-02",SourceRef:"TEST-ERP-CALENDAR-2026-02"}
	received:=time.Now().UTC().Format(time.RFC3339Nano)
	for day:=1;day<=28;day++{
		d:=time.Date(2026,2,day,8,0,0,0,loc)
		kind:="SHIFT"
		if d.Weekday()==time.Sunday {kind="REST"}
		workDate:=d.Format("2006-01-02")
		plan.Days=append(plan.Days,planDay{Date:workDate,Kind:kind})
		if kind!="SHIFT"{continue}
		end:=d.Add(8*time.Hour)
		if e:=v.service.db.Exec(`INSERT INTO oa_time_roster
			(tenant_id,person_id,work_date,shift_start,shift_end,unpaid_break_min,origin)
			VALUES(?,?,?,?,?,?,?)`,tenantID,plan.PersonID,workDate,d.Format(time.RFC3339),end.Format(time.RFC3339),30,"TEST-ERP-ROSTER");e!=nil{t.Fatal(e)}
		for _,k:=range []struct{dir string; at time.Time}{{"IN",d},{"OUT",end}}{
			id:=fmt.Sprintf("TEST-202602-%02d-%s",day,k.dir)
			if e:=v.service.db.Exec(`INSERT INTO oa_raw_punch
				(tenant_id,source,device_id,event_id,badge_id,person_id,direction,occurred_at_utc,received_at_utc)
				VALUES(?,?,?,?,?,?,?,?,?)`,tenantID,"TEST-ERP","SIM-GATE-01",id,plan.PersonID,plan.PersonID,
				k.dir,k.at.UTC().Format(time.RFC3339Nano),received);e!=nil{t.Fatal(e)}
		}
	}
	return plan
}

func TestV09TwoPersonTimeLockAndLateSourceDrift(t *testing.T) {
	v:=secured(t)
	plan:=setupV09ReadyMonth(t,v)
	hr:=signin(t,v,"hr")
	payroll:=signin(t,v,"payroll")
	employee:=signin(t,v,"employee")
	payload,e:=json.Marshal(plan)
	if e!=nil{t.Fatal(e)}
	x:=call(v.service,"POST","/v2/time/calendar/import",string(payload),employee,true)
	if x.Code!=403{t.Fatalf("employee imported plan: %d %s",x.Code,x.Body)}
	x=call(v.service,"POST","/v2/time/calendar/import",string(payload),hr,false)
	if x.Code!=403{t.Fatalf("plan accepted without CSRF: %d",x.Code)}
	x=call(v.service,"POST","/v2/time/calendar/import",string(payload),hr,true)
	if x.Code!=201{t.Fatalf("plan import: %d %s",x.Code,x.Body)}
	x=call(v.service,"GET","/v2/time/period/readiness?person_id=EMP-0012&period=2026-02","",hr,false)
	if x.Code!=200 || obj(t,x.Body.Bytes())["ready"]!=true{t.Fatalf("expected ready February: %d %s",x.Code,x.Body)}
	body:=`{"person_id":"EMP-0012","period":"2026-02"}`
	x=call(v.service,"POST","/v2/time/period/prepare",body,hr,true)
	if x.Code!=201 || obj(t,x.Body.Bytes())["status"]!="PREPARED"{
		t.Fatalf("HR preparation failed: %d %s",x.Code,x.Body)
	}
	x=call(v.service,"POST","/v2/time/period/approve",body,hr,true)
	if x.Code!=403{t.Fatalf("HR self/role approval accepted: %d %s",x.Code,x.Body)}
	x=call(v.service,"POST","/v2/time/period/approve",body,payroll,true)
	if x.Code!=201 || obj(t,x.Body.Bytes())["status"]!="LOCKED"{
		t.Fatalf("payroll independent lock failed: %d %s",x.Code,x.Body)
	}
	x=call(v.service,"GET","/v2/time/period/verify?person_id=EMP-0012&period=2026-02","",payroll,false)
	if x.Code!=200 || obj(t,x.Body.Bytes())["source_unchanged"]!=true{
		t.Fatalf("immediately changed snapshot: %d %s",x.Code,x.Body)
	}
	x=call(v.service,"POST","/v2/time/calendar/import",string(payload),hr,true)
	if x.Code!=409{t.Fatalf("locked calendar was edited: %d %s",x.Code,x.Body)}
	if e:=v.service.db.Exec(`UPDATE oa_time_roster SET unpaid_break_min=0
		WHERE tenant_id=? AND person_id='EMP-0012' AND work_date='2026-02-02'`,tenantID);e==nil{
		t.Fatal("locked roster SQL UPDATE accepted")
	}
	if e:=v.service.db.Exec(`UPDATE oa_time_month_close SET source_digest='forged'
		WHERE tenant_id=? AND person_id='EMP-0012' AND period='2026-02'`,tenantID);e==nil{
		t.Fatal("locked ledger SQL UPDATE accepted")
	}
	late:=`{"source":"TEST-LATE","device_id":"SIM-GATE-01","events":[{"event_id":"late-Feb-15","badge_id":"EMP-0012","direction":"IN","occurred_at":"2026-02-15T10:00:00+07:00"}]}`
	x=call(v.service,"POST","/v2/time/punches/import",late,hr,true)
	if x.Code!=201{t.Fatalf("late raw event not retained: %d %s",x.Code,x.Body)}
	x=call(v.service,"GET","/v2/time/period/verify?person_id=EMP-0012&period=2026-02","",payroll,false)
	if x.Code!=200 || obj(t,x.Body.Bytes())["source_unchanged"]!=false{
		t.Fatalf("late event not flagged for correction: %d %s",x.Code,x.Body)
	}
	closed,e:=v.service.one(`SELECT status,locked_by FROM oa_time_month_close WHERE tenant_id=? AND person_id=? AND period='2026-02'`,tenantID,plan.PersonID)
	if e!=nil || rowString(closed,"status")!="LOCKED" || rowString(closed,"locked_by")!="EMP-0007"{
		t.Fatalf("locked snapshot was altered: %v %v",closed,e)
	}
	x=call(v.service,"GET","/v2/time/period/readiness?person_id=EMP-0012&period=2026-02","",hr,false)
	if x.Code!=200 || obj(t,x.Body.Bytes())["ready"]!=false{
		t.Fatalf("unrostered late punch did not block readiness: %d %s",x.Code,x.Body)
	}
}
