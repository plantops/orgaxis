package timecore

import (
	"testing"
	"time"
)

var vn = time.FixedZone("Asia/Bangkok", 7*3600)

func at(v string) time.Time {
	t, e := time.ParseInLocation("2006-01-02 15:04", v, vn)
	if e != nil {
		panic(e)
	}
	return t
}
func p(id, kind, stamp string) Punch {
	return Punch{Source: "simulator", DeviceID: "GATE01", EventID: id, PersonID: "EMP-0012", Direction: kind, At: at(stamp)}
}
func standardShift() Shift {
	return Shift{PersonID: "EMP-0012", WorkDate: "2026-11-18", Start: at("2026-11-18 08:00"), End: at("2026-11-18 16:00"), UnpaidBreakMinutes: 30}
}
func TestNormalAndIdempotent(t *testing.T) {
	s := standardShift()
	events := []Punch{p("a", DirectionIn, "2026-11-18 08:00"), p("b", DirectionOut, "2026-11-18 16:00"), p("a", DirectionIn, "2026-11-18 08:00")}
	r, e := Evaluate(s, events, nil, nil)
	if e != nil || !r.CanClose || r.RawCount != 2 || r.ScheduledMinutes != 450 || r.CreditedRegularMinutes != 450 || r.ActualMinutes != 480 {
		t.Fatalf("result=%+v error=%v", r, e)
	}
}
func TestNightShiftAndOT(t *testing.T) {
	s := Shift{PersonID: "EMP-0012", WorkDate: "2026-11-18", Start: at("2026-11-18 22:00"), End: at("2026-11-19 06:00"), UnpaidBreakMinutes: 45}
	punches := []Punch{p("a", DirectionIn, "2026-11-18 22:00"), p("b", DirectionOut, "2026-11-19 07:00")}
	approval := []OTApproval{{Start: at("2026-11-19 06:00"), End: at("2026-11-19 07:00"), Kind: OTWeekday, Approved: true}}
	r, e := Evaluate(s, punches, nil, approval)
	if e != nil || !r.CanClose || r.CreditedRegularMinutes != 435 || r.ApprovedOTMinutes[OTWeekday] != 60 || r.UnapprovedOutsideMinutes != 0 {
		t.Fatalf("night=%+v err=%v", r, e)
	}
}
func TestUnknownAndMissingPunchBlockClose(t *testing.T) {
	s := standardShift()
	tests := [][]Punch{{p("a", DirectionIn, "2026-11-18 08:00")}, {p("a", "UNKNOWN", "2026-11-18 08:00")}, {p("x", DirectionOut, "2026-11-18 16:00")}}
	for _, tc := range tests {
		r, e := Evaluate(s, tc, nil, nil)
		if e != nil || r.CanClose || len(r.Exceptions) == 0 {
			t.Fatalf("should block %+v %v", r, e)
		}
	}
}
func TestUnauthorizedOvertimeNotPaid(t *testing.T) {
	r, e := Evaluate(standardShift(), []Punch{p("a", DirectionIn, "2026-11-18 08:00"), p("b", DirectionOut, "2026-11-18 18:00")}, nil, nil)
	if e != nil || r.CanClose || r.UnapprovedOutsideMinutes != 120 || r.ApprovedOTMinutes[OTWeekday] != 0 {
		t.Fatalf("OT=%+v %v", r, e)
	}
}
func TestApprovedPaidLeave(t *testing.T) {
	leave := Leave{Start: at("2026-11-18 08:00"), End: at("2026-11-18 16:00"), Approved: true, Paid: true}
	r, e := Evaluate(standardShift(), nil, []Leave{leave}, nil)
	// No device punches are required for a verified full-day paid leave.
	if e != nil || r.CreditedRegularMinutes != 450 || !r.CanClose {
		t.Fatalf("leave=%+v %v", r, e)
	}
}
func TestPayrollPreviewAndBlockedDay(t *testing.T) {
	s := standardShift()
	good, e := Evaluate(s, []Punch{p("a", DirectionIn, "2026-11-18 08:00"), p("b", DirectionOut, "2026-11-18 17:00")}, nil, []OTApproval{{Start: at("2026-11-18 16:00"), End: at("2026-11-18 17:00"), Kind: OTWeekday, Approved: true}})
	if e != nil || !good.CanClose {
		t.Fatalf("good: %+v %v", good, e)
	}
	pv, e := PreviewMonthlyPay("EMP-0012", "2026-11", 9000000, []Daily{good})
	if e != nil || pv.Blocked || pv.BasePayVND != 9000000 || pv.OTPayVND != 1800000 {
		t.Fatalf("preview=%+v %v", pv, e)
	}
	bad, _ := Evaluate(s, []Punch{p("z", DirectionIn, "2026-11-18 08:00")}, nil, nil)
	pv, e = PreviewMonthlyPay("EMP-0012", "2026-11", 9000000, []Daily{bad})
	if e != nil || !pv.Blocked {
		t.Fatalf("unclosed day unexpectedly paid: %+v %v", pv, e)
	}
}
func TestConflictingDuplicate(t *testing.T) {
	a := p("dup", DirectionIn, "2026-11-18 08:00")
	b := p("dup", DirectionOut, "2026-11-18 16:00")
	r, _ := Evaluate(standardShift(), []Punch{a, b}, nil, nil)
	if r.CanClose || r.RawCount != 1 {
		t.Fatalf("conflicting duplicate %+v", r)
	}
}
func TestNoOverlappingOTType(t *testing.T) {
	s := standardShift()
	punches := []Punch{p("a", DirectionIn, "2026-11-18 08:00"), p("b", DirectionOut, "2026-11-18 17:00")}
	ots := []OTApproval{{Start: at("2026-11-18 16:00"), End: at("2026-11-18 17:00"), Kind: OTWeekday, Approved: true}, {Start: at("2026-11-18 16:30"), End: at("2026-11-18 17:00"), Kind: OTRestDay, Approved: true}}
	r, _ := Evaluate(s, punches, nil, ots)
	if r.CanClose {
		t.Fatalf("overlapping OT types accepted: %+v", r)
	}
}
