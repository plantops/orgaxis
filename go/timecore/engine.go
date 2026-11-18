// Package timecore implements a small, auditable, deterministic attendance
// valuation core. It is NOT a statutory Vietnamese payroll calculator.
// Raw device events must be stored separately, without modification.
package timecore

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	DirectionIn  = "IN"
	DirectionOut = "OUT"
	OTWeekday    = "WEEKDAY"
	OTRestDay    = "REST_DAY"
	OTHoliday    = "HOLIDAY"
)

type Punch struct {
	Source    string    `json:"source"`
	DeviceID  string    `json:"device_id"`
	EventID   string    `json:"event_id"`
	PersonID  string    `json:"person_id"`
	Direction string    `json:"direction"`
	At        time.Time `json:"at"`
}
type Shift struct {
	PersonID           string    `json:"person_id"`
	WorkDate           string    `json:"work_date"` // Shift's start date in local site timezone.
	Start              time.Time `json:"start"`
	End                time.Time `json:"end"`
	UnpaidBreakMinutes int       `json:"unpaid_break_minutes"`
}
type Leave struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Approved bool      `json:"approved"`
	Paid     bool      `json:"paid"`
}
type OTApproval struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Kind     string    `json:"kind"`
	Approved bool      `json:"approved"`
}
type Pair struct{ Start, End time.Time }
type Daily struct {
	PersonID                 string         `json:"person_id"`
	WorkDate                 string         `json:"work_date"`
	ScheduledMinutes         int            `json:"scheduled_minutes"`
	ActualMinutes            int            `json:"actual_minutes"`
	CreditedRegularMinutes   int            `json:"credited_regular_minutes"`
	ApprovedOTMinutes        map[string]int `json:"approved_ot_minutes"`
	UnapprovedOutsideMinutes int            `json:"unapproved_outside_minutes"`
	LateMinutes              int            `json:"late_minutes"`
	EarlyMinutes             int            `json:"early_minutes"`
	RawCount                 int            `json:"raw_count"`
	Exceptions               []string       `json:"exceptions"`
	CanClose                 bool           `json:"can_close"`
}
type PayPreview struct {
	Period                 string         `json:"period"`
	PersonID               string         `json:"person_id"`
	ScheduledMinutes       int            `json:"scheduled_minutes"`
	CreditedRegularMinutes int            `json:"credited_regular_minutes"`
	OTMinutes              map[string]int `json:"ot_minutes"`
	BasePayVND             int64          `json:"base_pay_vnd"`
	OTPayVND               int64          `json:"ot_pay_vnd"`
	GrossPreviewVND        int64          `json:"gross_preview_vnd"`
	Blocked                bool           `json:"blocked"`
	Exceptions             []string       `json:"exceptions"`
	Note                   string         `json:"note"`
}

func overlap(a, b, c, d time.Time) int {
	if a.Before(c) {
		a = c
	}
	if b.After(d) {
		b = d
	}
	if !a.Before(b) {
		return 0
	}
	return int(b.Sub(a) / time.Minute)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func mergeSegments(xs []Pair) []Pair {
	if len(xs) == 0 {
		return nil
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].Start.Before(xs[j].Start) })
	out := []Pair{xs[0]}
	for _, p := range xs[1:] {
		last := &out[len(out)-1]
		if !p.Start.After(last.End) {
			if p.End.After(last.End) {
				last.End = p.End
			}
		} else {
			out = append(out, p)
		}
	}
	return out
}
func minutes(xs []Pair) int {
	n := 0
	for _, p := range mergeSegments(xs) {
		n += int(p.End.Sub(p.Start) / time.Minute)
	}
	return n
}
func validWindow(a, b time.Time) bool { return !a.IsZero() && !b.IsZero() && b.After(a) }
func validKind(k string) bool         { return k == OTWeekday || k == OTRestDay || k == OTHoliday }

// Evaluate pairs explicit IN/OUT punches for one scheduled shift. Neither
// duration nor pay entitlement is inferred from an unpaired or undirected event.
// Every exception makes CanClose false. Corrections require new evidence, not
// mutation of historical raw events.
func Evaluate(shift Shift, punches []Punch, leaves []Leave, approvals []OTApproval) (Daily, error) {
	if !validWindow(shift.Start, shift.End) || shift.End.Sub(shift.Start) > 18*time.Hour {
		return Daily{}, errors.New("invalid scheduled shift")
	}
	total := int(shift.End.Sub(shift.Start) / time.Minute)
	if shift.UnpaidBreakMinutes < 0 || shift.UnpaidBreakMinutes >= total {
		return Daily{}, errors.New("invalid break")
	}
	expected := total - shift.UnpaidBreakMinutes
	result := Daily{PersonID: shift.PersonID, WorkDate: shift.WorkDate, ScheduledMinutes: expected, ApprovedOTMinutes: map[string]int{OTWeekday: 0, OTRestDay: 0, OTHoliday: 0}, Exceptions: []string{}}
	events := append([]Punch(nil), punches...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	unique := map[string]Punch{}
	raw := make([]Punch, 0, len(events))
	for _, p := range events {
		if p.PersonID != shift.PersonID {
			result.Exceptions = append(result.Exceptions, "PUNCH_OTHER_PERSON")
			continue
		}
		if p.At.IsZero() || p.DeviceID == "" || p.EventID == "" || p.Source == "" {
			result.Exceptions = append(result.Exceptions, "INVALID_PUNCH_SOURCE")
			continue
		}
		if p.Direction != DirectionIn && p.Direction != DirectionOut {
			result.Exceptions = append(result.Exceptions, "UNKNOWN_PUNCH_DIRECTION")
			continue
		}
		key := strings.Join([]string{p.Source, p.DeviceID, p.EventID}, "\x00")
		if prev, ok := unique[key]; ok {
			if prev.At != p.At || prev.Direction != p.Direction || prev.PersonID != p.PersonID {
				result.Exceptions = append(result.Exceptions, "CONFLICTING_DUPLICATE_EVENT")
			}
			continue
		}
		unique[key] = p
		// Collect only events from this shift's bounded observation window.
		if p.At.Before(shift.Start.Add(-4*time.Hour)) || p.At.After(shift.End.Add(6*time.Hour)) {
			continue
		}
		raw = append(raw, p)
	}
	result.RawCount = len(raw)
	pairs := make([]Pair, 0)
	var opened *Punch
	for i := range raw {
		p := &raw[i]
		if p.Direction == DirectionIn {
			if opened != nil {
				result.Exceptions = append(result.Exceptions, "CONSECUTIVE_IN")
			} else {
				opened = p
			}
		} else {
			if opened == nil {
				result.Exceptions = append(result.Exceptions, "OUT_WITHOUT_IN")
			} else {
				if !p.At.After(opened.At) {
					result.Exceptions = append(result.Exceptions, "NONPOSITIVE_INTERVAL")
				} else {
					pairs = append(pairs, Pair{opened.At, p.At})
				}
				opened = nil
			}
		}
	}
	if opened != nil {
		result.Exceptions = append(result.Exceptions, "MISSING_OUT")
	}
	if len(pairs) == 0 && len(raw) == 0 {
		result.Exceptions = append(result.Exceptions, "NO_PUNCH")
	}
	// Overlapping pairs are not valid evidence for additive worked hours.
	joined := mergeSegments(append([]Pair(nil), pairs...))
	if len(joined) < len(pairs) {
		result.Exceptions = append(result.Exceptions, "OVERLAPPING_PAIRS")
	}
	result.ActualMinutes = minutes(pairs)
	regularSegments := make([]Pair, 0, len(pairs)+len(leaves))
	for _, p := range pairs {
		a, b := p.Start, p.End
		if a.Before(shift.Start) {
			a = shift.Start
		}
		if b.After(shift.End) {
			b = shift.End
		}
		if a.Before(b) {
			regularSegments = append(regularSegments, Pair{a, b})
		}
	}
	firstIn, lastOut := time.Time{}, time.Time{}
	for _, p := range pairs {
		if firstIn.IsZero() || p.Start.Before(firstIn) {
			firstIn = p.Start
		}
		if p.End.After(lastOut) {
			lastOut = p.End
		}
	}
	if !firstIn.IsZero() && firstIn.After(shift.Start) {
		result.LateMinutes = min(expected, int(firstIn.Sub(shift.Start)/time.Minute))
	}
	if !lastOut.IsZero() && lastOut.Before(shift.End) {
		result.EarlyMinutes = min(expected, int(shift.End.Sub(lastOut)/time.Minute))
	}
	for _, l := range leaves {
		if !validWindow(l.Start, l.End) {
			result.Exceptions = append(result.Exceptions, "INVALID_LEAVE")
			continue
		}
		if l.Approved && l.Paid {
			a, b := l.Start, l.End
			if a.Before(shift.Start) {
				a = shift.Start
			}
			if b.After(shift.End) {
				b = shift.End
			}
			if a.Before(b) {
				regularSegments = append(regularSegments, Pair{a, b})
			}
		}
	}
	// A configured unpaid break is subtracted once from the union of worked and
	// approved paid-leave intervals. A separate break schedule is future scope.
	result.CreditedRegularMinutes = min(expected, max(0, minutes(regularSegments)-shift.UnpaidBreakMinutes))
	// Approved all-day paid leave needs no biometric event. Partial-day leave
	// still requires attendance for the remaining working time.
	if len(raw) == 0 && result.CreditedRegularMinutes == expected {
		clean := result.Exceptions[:0]
		for _, reason := range result.Exceptions {
			if reason != "NO_PUNCH" {
				clean = append(clean, reason)
			}
		}
		result.Exceptions = clean
	}
	approvalSlots := map[string][]Pair{OTWeekday: {}, OTRestDay: {}, OTHoliday: {}}
	for _, a := range approvals {
		if !a.Approved {
			continue
		}
		if !validWindow(a.Start, a.End) || !validKind(a.Kind) {
			result.Exceptions = append(result.Exceptions, "INVALID_OT_APPROVAL")
			continue
		}
		approvalSlots[a.Kind] = append(approvalSlots[a.Kind], Pair{a.Start, a.End})
	}
	// Concurrent OT entitlements with different day types must be resolved by HR.
	otTypes := []string{OTWeekday, OTRestDay, OTHoliday}
	for i, k := range otTypes {
		for _, a := range approvalSlots[k] {
			for _, j := range otTypes[i+1:] {
				for _, b := range approvalSlots[j] {
					if overlap(a.Start, a.End, b.Start, b.End) > 0 {
						result.Exceptions = append(result.Exceptions, "CONFLICTING_OT_TYPES")
					}
				}
			}
		}
	}
	allOT := make([]Pair, 0)
	outside := 0
	for _, p := range pairs {
		outside += int(p.End.Sub(p.Start)/time.Minute) - overlap(p.Start, p.End, shift.Start, shift.End)
	}
	for _, k := range otTypes {
		segments := make([]Pair, 0)
		for _, ap := range mergeSegments(approvalSlots[k]) {
			for _, p := range pairs {
				a, b := p.Start, p.End
				if a.Before(ap.Start) {
					a = ap.Start
				}
				if b.After(ap.End) {
					b = ap.End
				}
				if a.Before(b) {
					if a.Before(shift.Start) {
						end := b
						if end.After(shift.Start) {
							end = shift.Start
						}
						if a.Before(end) {
							segments = append(segments, Pair{a, end})
						}
					}
					if b.After(shift.End) {
						start := a
						if start.Before(shift.End) {
							start = shift.End
						}
						if start.Before(b) {
							segments = append(segments, Pair{start, b})
						}
					}
				}
			}
		}
		segments = mergeSegments(segments)
		result.ApprovedOTMinutes[k] = minutes(segments)
		allOT = append(allOT, segments...)
	}
	result.UnapprovedOutsideMinutes = max(0, outside-minutes(allOT))
	if result.UnapprovedOutsideMinutes > 0 {
		result.Exceptions = append(result.Exceptions, "UNAPPROVED_OUTSIDE_TIME")
	}
	result.CanClose = len(result.Exceptions) == 0
	return result, nil
}

// PreviewMonthlyPay is intentionally a gross-only illustrative calculation.
// It does not calculate night-work premiums, payroll tax, insurance, benefits,
// statutory overtime ceilings, retro-pay or rounding under labor contracts.
func PreviewMonthlyPay(person, period string, monthlyBase int64, days []Daily) (PayPreview, error) {
	if monthlyBase < 0 {
		return PayPreview{}, errors.New("negative base pay")
	}
	result := PayPreview{PersonID: person, Period: period, OTMinutes: map[string]int{OTWeekday: 0, OTRestDay: 0, OTHoliday: 0}, Exceptions: []string{}, Note: "SYNTHETIC GROSS PREVIEW ONLY; requires HR verification before payroll posting"}
	for _, d := range days {
		if d.PersonID != person || !strings.HasPrefix(d.WorkDate, period) {
			return PayPreview{}, fmt.Errorf("timesheet mismatch: %s", d.WorkDate)
		}
		result.ScheduledMinutes += d.ScheduledMinutes
		result.CreditedRegularMinutes += d.CreditedRegularMinutes
		for k, v := range d.ApprovedOTMinutes {
			if validKind(k) {
				result.OTMinutes[k] += v
			}
		}
		if !d.CanClose {
			result.Exceptions = append(result.Exceptions, fmt.Sprintf("%s: %s", d.WorkDate, strings.Join(d.Exceptions, ",")))
		}
	}
	if result.ScheduledMinutes == 0 {
		return PayPreview{}, errors.New("missing schedule")
	}
	result.Blocked = len(result.Exceptions) > 0
	baseMinutes := min(result.ScheduledMinutes, result.CreditedRegularMinutes)
	result.BasePayVND = int64(math.Round(float64(monthlyBase) * float64(baseMinutes) / float64(result.ScheduledMinutes)))
	// Monthly base / monthly normal minutes = rate per minute.
	perMinute := float64(monthlyBase) / float64(result.ScheduledMinutes)
	sum := float64(result.OTMinutes[OTWeekday])*1.5 + float64(result.OTMinutes[OTRestDay])*2 + float64(result.OTMinutes[OTHoliday])*3
	result.OTPayVND = int64(math.Round(sum * perMinute))
	result.GrossPreviewVND = result.BasePayVND + result.OTPayVND
	return result, nil
}
