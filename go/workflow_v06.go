package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Workdays are the company's synthetic demo calendar: Monday-Saturday;
// public holidays and employee-specific shifts are deliberately not applied.
func demoWorkdays(firstDay, lastDay string) (int, error) {
	a, e := time.Parse("2006-01-02", firstDay)
	if e != nil {
		return 0, fail(422, "Invalid start date")
	}
	b, e := time.Parse("2006-01-02", lastDay)
	if e != nil || b.Before(a) || b.Sub(a).Hours()/24 > 180 {
		return 0, fail(422, "Invalid end date or exceeds 180 days")
	}
	n := 0
	for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Sunday {
			n++
		}
	}
	return n, nil
}
func kaizenReward(net int64, category string, level int, annex03 bool) (string, int64) {
	if category == "NONFINANCIAL" {
		amounts := []int64{0, 1_000_000, 3_000_000, 8_000_000, 25_000_000}
		if level < 1 || level > 4 {
			return "INVALID", 0
		}
		return fmt.Sprintf("NF%d", level), amounts[level]
	}
	var tier string
	var amount int64
	defer func() { _ = tier; _ = amount }()
	return kaizenFinancial(net, annex03)
}
func kaizenFinancial(net int64, annex03 bool) (string, int64) {
	var tier string
	var amount int64
	switch {
	case net <= 0:
		tier, amount = "T1", 300_000
	case net < 20_000_000:
		tier, amount = "T2", 1_000_000
	case net < 50_000_000:
		tier, amount = "T3", 2_500_000
	case net < 150_000_000:
		tier, amount = "T4", 5_000_000
	case net < 300_000_000:
		tier, amount = "T5", 10_000_000
	case net < 500_000_000:
		tier, amount = "T6", 20_000_000
	default:
		tier, amount = "T7", int64(math.Min(float64(net)*.06, 100_000_000))
	}
	if annex03 {
		amount = int64(math.Min(float64(amount)*1.2, 100_000_000))
	}
	return tier, amount
}

func (s *Service) findAssignee(roleName, exclude string) (string, error) {
	rows, e := s.rows(`SELECT person_id FROM oa_user WHERE tenant_id=? AND role=? AND active=1 AND person_id<>? ORDER BY username LIMIT 1`, tenantID, roleName, exclude)
	if e != nil {
		return "", e
	}
	if len(rows) == 0 {
		return "", fail(409, "No configured approver for role "+roleName)
	}
	return rowString(rows[0], "person_id"), nil
}
func (s *Service) approvalSteps(subject, date, kind string, days int) ([]Row, error) {
	steps := []Row{}
	manager, e := s.manager(subject, date)
	if e != nil {
		return nil, e
	}
	if manager["status"] != "resolved" {
		return nil, fail(409, "Reporting line is unresolved at request date")
	}
	p, ok := manager["manager"].(Row)
	if !ok {
		return nil, fail(409, "Missing line manager")
	}
	managerID := rowString(p, "person_id")
	if managerID == subject {
		return nil, fail(409, "Self-approval is not allowed")
	}
	steps = append(steps, Row{"role": "LINE_MANAGER", "assignee": managerID})
	short := s.policyInt("leave_direct_days", 2)
	medium := s.policyInt("leave_hr_days", 5)
	if kind == "LEAVE" && days <= short {
		return steps, nil
	}
	hr, e := s.findAssignee("HR", subject)
	if e != nil {
		return nil, e
	}
	if hr != managerID {
		steps = append(steps, Row{"role": "HR", "assignee": hr})
	}
	if kind == "LEAVE" && days <= medium {
		return steps, nil
	}
	director, e := s.findAssignee("DIRECTOR", subject)
	if e != nil {
		return nil, e
	}
	if director != managerID && director != hr {
		steps = append(steps, Row{"role": "DIRECTOR", "assignee": director})
	}
	return steps, nil
}
func (s *Service) openV6Case(p *Principal, r *http.Request) (any, error) {
	var v struct {
		Type        string `json:"type"`
		SubjectID   string `json:"subject_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Start       string `json:"start"`
		End         string `json:"end"`
		NetBenefit  int64  `json:"net_benefit"`
		ImpactLevel int    `json:"impact_level"`
		Annex03     bool   `json:"annex03"`
	}
	if e := bodyJSON(r, &v); e != nil {
		return nil, e
	}
	kind := strings.ToUpper(strings.TrimSpace(v.Type))
	if !in(kind, "LEAVE", "BONUS", "DISCIPLINE") {
		return nil, fail(422, "Invalid case type")
	}
	subject := p.PersonID
	if kind != "LEAVE" {
		subject = strings.TrimSpace(v.SubjectID)
		if subject == "" {
			return nil, fail(422, "Subject is required")
		}
		if !role(p, "SUPERVISOR", "DIRECTOR", "ADMIN") || !s.subordinate(p.PersonID, subject, time.Now().Format("2006-01-02")) {
			return nil, fail(403, "Only an eligible line manager may initiate this case")
		}
	} else if v.SubjectID != "" && v.SubjectID != p.PersonID {
		return nil, fail(403, "Leave must be submitted by the logged-in employee")
	}
	if e := s.requirePerson(subject); e != nil {
		return nil, e
	}
	title := strings.TrimSpace(v.Title)
	if title == "" {
		switch kind {
		case "LEAVE":
			title = "Nghỉ phép"
		case "BONUS":
			title = "Đề xuất khen thưởng"
		default:
			title = "Hồ sơ xem xét kỷ luật"
		}
	}
	if len(title) > 160 || len(v.Description) > 2000 {
		return nil, fail(422, "Text too long")
	}
	d := time.Now().Format("2006-01-02")
	days := 0
	if kind == "LEAVE" {
		var e error
		days, e = demoWorkdays(v.Start, v.End)
		if e != nil {
			return nil, e
		}
		if days == 0 {
			return nil, fail(422, "No scheduled workdays")
		}
		d = v.Start
	}
	if kind == "BONUS" && (v.NetBenefit < 0 || v.NetBenefit > 100_000_000_000) {
		return nil, fail(422, "Net benefit out of range")
	}
	if kind == "DISCIPLINE" && len(strings.TrimSpace(v.Description)) < 12 {
		return nil, fail(422, "Incident statement and evidence reference required")
	}
	if kind != "LEAVE" {
		d = time.Now().Format("2006-01-02")
	}
	steps, e := s.approvalSteps(subject, d, kind, days)
	if e != nil {
		return nil, e
	}
	// For supervisor-originated bonus/discipline, the opener's LINE_MANAGER step
	// is an initiation decision already made by the originator, not self-approval.
	if kind != "LEAVE" && len(steps) > 0 && steps[0]["assignee"] == p.PersonID {
		steps = steps[1:]
	}
	if len(steps) == 0 {
		return nil, fail(409, "No subsequent independent reviewer")
	}
	tier := ""
	var estimate int64
	if kind == "BONUS" {
		tier, estimate = kaizenReward(v.NetBenefit, v.Category, v.ImpactLevel, v.Annex03)
		if tier == "INVALID" {
			return nil, fail(422, "Nonfinancial impact level must be 1–4")
		}
	}
	caseID := uid("HRC")
	if e := s.db.Exec("BEGIN IMMEDIATE"); e != nil {
		return nil, e
	}
	done := false
	defer func() {
		if !done {
			_ = s.db.Exec("ROLLBACK")
		}
	}()
	if kind == "LEAVE" {
		overlaps, e := s.one(`SELECT COUNT(*) AS n FROM oa_case WHERE tenant_id=? AND subject_id=? AND case_type='LEAVE' AND status NOT IN ('REJECTED','CANCELLED') AND start_date<=? AND end_date>=?`, tenantID, subject, v.End, v.Start)
		if e != nil {
			return nil, e
		}
		if intval(overlaps["n"]) > 0 {
			return nil, fail(409, "Leave dates overlap an existing case")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if e := s.db.Exec(`INSERT INTO oa_case(id,tenant_id,subject_id,opened_by,case_type,title,description,category,start_date,end_date,workdays,net_benefit,impact_level,annex03,reward_estimate,reward_tier,stage,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'OPEN',?)`, caseID, tenantID, subject, p.PersonID, kind, title, v.Description, v.Category, nullableTime(v.Start), nullableTime(v.End), days, v.NetBenefit, v.ImpactLevel, boolInt(v.Annex03), estimate, tier, rowString(steps[0], "role"), now); e != nil {
		return nil, e
	}
	for i, step := range steps {
		state := "BLOCKED"
		if i == 0 {
			state = "WAITING"
		}
		if e := s.db.Exec(`INSERT INTO oa_case_step(id,case_id,seq,role,assignee_id,status) VALUES(?,?,?,?,?,?)`, uid("STP"), caseID, i+1, step["role"], step["assignee"], state); e != nil {
			return nil, e
		}
	}
	if e := auditV6(s, p.PersonID, "CASE_OPEN", caseID, map[string]any{"type": kind, "subject": subject, "steps": steps}); e != nil {
		return nil, e
	}
	if e := s.db.Exec("COMMIT"); e != nil {
		return nil, e
	}
	done = true
	return map[string]any{"id": caseID, "status": "OPEN", "route": steps, "workdays": days, "reward_tier": tier, "estimate": estimate}, nil
}
func (s *Service) caseDecision(p *Principal, r *http.Request, id string) (any, error) {
	var v struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := bodyJSON(r, &v); err != nil {
		return nil, err
	}
	v.Decision = strings.ToUpper(v.Decision)
	if !in(v.Decision, "APPROVE", "REJECT", "NEED_INFO") {
		return nil, fail(422, "Invalid decision")
	}
	caseRow, e := s.one(`SELECT c.*,ss.id AS step_id,ss.seq,ss.role AS required_role,ss.assignee_id FROM oa_case c JOIN oa_case_step ss ON ss.case_id=c.id AND ss.status='WAITING' WHERE c.id=? AND c.tenant_id=? AND c.status='OPEN'`, id, tenantID)
	if e != nil {
		return nil, e
	}
	if caseRow == nil {
		return nil, fail(409, "Case is not waiting for approval")
	}
	if p.PersonID == rowString(caseRow, "subject_id") {
		return nil, fail(403, "Self-approval is not allowed")
	}
	if p.PersonID != rowString(caseRow, "assignee_id") {
		return nil, fail(403, "Not the assigned reviewer")
	}
	stageRole := rowString(caseRow, "required_role")
	authorized := false
	switch stageRole {
	case "LINE_MANAGER":
		authorized = role(p, "SUPERVISOR", "DIRECTOR", "ADMIN")
	case "HR":
		authorized = role(p, "HR")
	case "DIRECTOR":
		authorized = role(p, "DIRECTOR")
	}
	if !authorized {
		return nil, fail(403, "Role no longer has decision authority")
	}

	if len(v.Note) > 1000 {
		return nil, fail(422, "Decision note too long")
	}
	if e := s.db.Exec("BEGIN IMMEDIATE"); e != nil {
		return nil, e
	}
	done := false
	defer func() {
		if !done {
			_ = s.db.Exec("ROLLBACK")
		}
	}()
	now := time.Now().UTC().Format(time.RFC3339)
	count, e := s.one(`SELECT COUNT(*) AS n FROM oa_case_step WHERE id=? AND status='WAITING' AND assignee_id=?`, caseRow["step_id"], p.PersonID)
	if e != nil {
		return nil, e
	}
	if intval(count["n"]) != 1 {
		return nil, fail(409, "Case already processed")
	}
	if err := s.db.Exec(`UPDATE oa_case_step SET status=?,decided_by=?,decision_at=?,note=? WHERE id=? AND status='WAITING'`, v.Decision, p.PersonID, now, v.Note, caseRow["step_id"]); err != nil {
		return nil, err
	}
	status := "OPEN"
	stage := ""
	if v.Decision == "REJECT" {
		status = "REJECTED"
	} else if v.Decision == "NEED_INFO" {
		status = "NEEDS_INFO"
	} else {
		next, e := s.one(`SELECT id,role FROM oa_case_step WHERE case_id=? AND seq>? ORDER BY seq LIMIT 1`, id, caseRow["seq"])
		if e != nil {
			return nil, e
		}
		if next == nil {
			status = "APPROVED"
		} else {
			stage = rowString(next, "role")
			if e := s.db.Exec(`UPDATE oa_case_step SET status='WAITING' WHERE id=? AND status='BLOCKED'`, next["id"]); e != nil {
				return nil, e
			}
		}
	}
	if err := s.db.Exec(`UPDATE oa_case SET status=?,stage=?,closed_at=? WHERE id=? AND status='OPEN'`, status, stage, func() any {
		if status == "OPEN" {
			return nil
		}
		return now
	}(), id); err != nil {
		return nil, err
	}
	if err := auditV6(s, p.PersonID, "CASE_DECISION", id, map[string]any{"decision": v.Decision, "note": v.Note}); err != nil {
		return nil, err
	}
	if e := s.db.Exec("COMMIT"); e != nil {
		return nil, e
	}
	done = true
	return map[string]any{"id": id, "status": status, "stage": stage}, nil
}
func (s *Service) listCases(p *Principal, r *http.Request) (any, error) {
	kind := strings.ToUpper(r.URL.Query().Get("type"))
	if kind != "" && !in(kind, "LEAVE", "BONUS", "DISCIPLINE") {
		return nil, fail(422, "Invalid type")
	}
	// List is scoped in SQL: subject, opener, assigned reviewer, HR visibility only
	// for active HR steps. HR cannot browse unrelated confidential disciplinary cases.
	entries, e := s.rows(`SELECT c.id,c.subject_id,p.display_name AS subject_name,c.opened_by,c.case_type,c.title,c.description,c.category,c.start_date,c.end_date,c.workdays,c.net_benefit,c.reward_estimate,c.reward_tier,c.status,c.stage,c.created_at,ss.assignee_id AS current_reviewer,ss.role AS current_role FROM oa_case c JOIN person p ON p.id=c.subject_id LEFT JOIN oa_case_step ss ON ss.case_id=c.id AND ss.status='WAITING' WHERE c.tenant_id=? AND (?='' OR c.case_type=?) AND (c.subject_id=? OR c.opened_by=? OR EXISTS(SELECT 1 FROM oa_case_step x WHERE x.case_id=c.id AND x.assignee_id=?)) ORDER BY c.created_at DESC LIMIT 500`, tenantID, kind, kind, p.PersonID, p.PersonID, p.PersonID)
	if e != nil {
		return nil, e
	}
	for _, x := range entries {
		if x["case_type"] == "DISCIPLINE" && x["subject_id"] == p.PersonID && x["opened_by"] != p.PersonID { // Employee sees status, not investigation notes.
			x["description"] = "Restricted until authorized disclosure"
		}
	}
	return map[string]any{"items": entries, "count": len(entries)}, nil
}
func (s *Service) caseHistory(p *Principal, id string) (any, error) {
	c, e := s.one(`SELECT * FROM oa_case WHERE id=? AND tenant_id=?`, id, tenantID)
	if e != nil {
		return nil, e
	}
	if c == nil {
		return nil, fail(404, "Case not found")
	}
	allowed := rowString(c, "subject_id") == p.PersonID || rowString(c, "opened_by") == p.PersonID
	steps, e := s.rows(`SELECT seq,role,assignee_id,status,decided_by,decision_at,note FROM oa_case_step WHERE case_id=? ORDER BY seq`, id)
	if e != nil {
		return nil, e
	}
	for _, st := range steps {
		if st["assignee_id"] == p.PersonID {
			allowed = true
		}
	}
	if !allowed {
		return nil, fail(404, "Case not found")
	}
	if c["case_type"] == "DISCIPLINE" && c["subject_id"] == p.PersonID && c["opened_by"] != p.PersonID {
		c["description"] = "Restricted until authorized disclosure"
		for _, st := range steps {
			st["note"] = "Restricted"
		}
	}
	return map[string]any{"case": c, "steps": steps}, nil
}
func (s *Service) v6API(p *Principal, r *http.Request) (any, error) {
	path := r.URL.Path
	date, e := argDate(r)
	if e != nil {
		return nil, e
	}
	if strings.HasPrefix(path, "/v2/time/") {
		return s.attendanceAPI(p, r)
	}
	switch {
	case path == "/v2/modules" && r.Method == "GET":
		return moduleInventoryV11(r)
	case path == "/v2/org/full" && r.Method == "GET":
		return s.orgFull(date, r.URL.Query().Get("unit"), r.URL.Query().Get("team"))
	case path == "/v2/cases" && r.Method == "GET":
		return s.listCases(p, r)
	case path == "/v2/cases" && r.Method == "POST":
		return s.openV6Case(p, r)
	case strings.HasPrefix(path, "/v2/cases/") && strings.HasSuffix(path, "/resubmit") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v2/cases/"), "/resubmit")
		return s.resubmitV6(p, id)
	case strings.HasPrefix(path, "/v2/cases/") && r.Method == "GET":
		return s.caseHistory(p, strings.TrimPrefix(path, "/v2/cases/"))
	case strings.HasPrefix(path, "/v2/cases/") && strings.HasSuffix(path, "/decision") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v2/cases/"), "/decision")
		return s.caseDecision(p, r, id)
	case path == "/v2/payroll/year" && r.Method == "GET":
		if !role(p, "PAYROLL", "ADMIN", "DIRECTOR") {
			return nil, fail(403, "Payroll permission required")
		}
		return s.payrollAnnual()
	case strings.HasPrefix(path, "/v2/payroll/person/") && r.Method == "GET":
		person := strings.TrimPrefix(path, "/v2/payroll/person/")
		if !s.canSeePayroll(p, person) {
			return nil, fail(403, "No payroll permission")
		}
		return s.payrollPersonAnnual(person)
	case path == "/v2/people/me" && r.Method == "GET":
		return s.personDetail(p.PersonID, date)
	case strings.HasPrefix(path, "/v2/people/") && r.Method == "GET":
		person := strings.TrimPrefix(path, "/v2/people/")
		if !s.canSeePerson(p, person, date) {
			return nil, fail(403, "Out of scope")
		}
		return s.personDetail(person, date)
	case path == "/v2/users" && r.Method == "GET":
		if !role(p, "ADMIN") {
			return nil, fail(403, "RBAC administration required")
		}
		rows, e := s.rows(`SELECT username,person_id,role,active,created_at FROM oa_user WHERE tenant_id=? ORDER BY username`, tenantID)
		return map[string]any{"items": rows}, e
	case path == "/v2/users" && r.Method == "POST":
		if !role(p, "ADMIN") {
			return nil, fail(403, "RBAC administration required")
		}
		var v struct {
			PersonID string `json:"person_id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		}
		if e := bodyJSON(r, &v); e != nil {
			return nil, e
		}
		v.Username = strings.ToLower(strings.TrimSpace(v.Username))
		v.Role = strings.ToUpper(strings.TrimSpace(v.Role))
		if len(v.Username) < 3 || len(v.Username) > 40 || strings.ContainsAny(v.Username, ` /\"'<>;`) || !in(v.Role, "EMPLOYEE", "SUPERVISOR", "HR", "PAYROLL", "DIRECTOR", "ADMIN") {
			return nil, fail(422, "Invalid user")
		}
		if e := s.requirePerson(v.PersonID); e != nil {
			return nil, e
		}
		pwd := token()[:24]
		salt := token()[:32]
		hash := derive(pwd, salt, 100000)
		if e := s.db.Exec(`INSERT INTO oa_user(id,tenant_id,username,person_id,role,salt,passhash,created_at) VALUES(?,?,?,?,?,?,?,?)`, uid("USR"), tenantID, v.Username, v.PersonID, v.Role, salt, hash, time.Now().UTC().Format(time.RFC3339)); e != nil {
			return nil, fail(409, "Username may already exist")
		}
		_ = auditV6(s, p.PersonID, "USER_CREATE", v.Username, map[string]any{"person_id": v.PersonID, "role": v.Role})
		return map[string]any{"username": v.Username, "role": v.Role, "password_once": pwd, "warning": "Synthetic demo account, do not use for real HR"}, nil
	case path == "/v2/policies" && r.Method == "GET":
		rows, e := s.rows(`SELECT key,value FROM oa_policy WHERE tenant_id=? ORDER BY key`, tenantID)
		return map[string]any{"items": rows}, e
	case path == "/v2/policies" && r.Method == "POST":
		if !role(p, "ADMIN") {
			return nil, fail(403, "Only admin can change policy")
		}
		var v struct {
			DirectDays int `json:"direct_days"`
			HRDays     int `json:"hr_days"`
		}
		if e := bodyJSON(r, &v); e != nil {
			return nil, e
		}
		if v.DirectDays < 1 || v.HRDays <= v.DirectDays || v.HRDays > 30 {
			return nil, fail(422, "Invalid escalation thresholds")
		}
		if e := s.db.Exec(`UPDATE oa_policy SET value=? WHERE tenant_id=? AND key='leave_direct_days'`, strconv.Itoa(v.DirectDays), tenantID); e != nil {
			return nil, e
		}
		if e := s.db.Exec(`UPDATE oa_policy SET value=? WHERE tenant_id=? AND key='leave_hr_days'`, strconv.Itoa(v.HRDays), tenantID); e != nil {
			return nil, e
		}
		_ = auditV6(s, p.PersonID, "POLICY_UPDATE", "leave", v)
		return map[string]any{"direct_days": v.DirectDays, "hr_days": v.HRDays}, nil
	case path == "/v2/audit" && r.Method == "GET":
		if !role(p, "ADMIN") {
			return nil, fail(403, "Audit permission required")
		}
		rows, e := s.rows(`SELECT actor_id,event,object_id,occurred_at FROM oa_audit ORDER BY occurred_at DESC LIMIT 150`)
		return map[string]any{"events": rows}, e
	default:
		return nil, fail(404, "Unknown endpoint")
	}
}
func (s *Service) payrollAnnual() (any, error) {
	rows, e := s.rows(`SELECT period,COUNT(*) AS headcount,SUM(base_pay) AS base,SUM(allowance) AS allowances,SUM(overtime) AS overtime,SUM(attendance_bonus+performance_bonus) AS bonuses,SUM(gross) AS gross,SUM(employee_insurance) AS insurance,SUM(pit_estimate) AS pit,SUM(net_estimate) AS net FROM payroll_demo_ledger WHERE tenant_id=? AND period LIKE '2026-%' GROUP BY period ORDER BY period`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"months": rows, "count": len(rows), "note": "SYNTHETIC 2026, not a legally verified payroll, insurance or PIT computation"}, nil
}
func (s *Service) payrollPersonAnnual(person string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	rows, e := s.rows(`SELECT period,base_pay,allowance,overtime,attendance_bonus,performance_bonus,gross,employee_insurance,other_deduction,taxable_income,pit_estimate,net_estimate,rule_version FROM payroll_demo_ledger WHERE tenant_id=? AND person_id=? AND period LIKE '2026-%' ORDER BY period`, tenantID, person)
	if e != nil {
		return nil, e
	}
	return map[string]any{"person_id": person, "items": rows, "count": len(rows), "legal_status": "simulation_only"}, nil
}
func (s *Service) orgFull(date, unit, team string) (any, error) {
	units, e := s.blankOrg(date)
	if e != nil {
		return nil, e
	}
	// Query full effective-dated graph first, even when the caller filters a unit.
	// Managers outside a department are still available for reporting resolution.
	rows, e := s.rows(`SELECT a.person_id,a.position_id,p.display_name,p.nationality,po.code AS position_code,po.title AS position_title,u.id AS unit_id,u.code AS unit_code,u.name AS unit_name,pr.supervisor_position_id FROM assignment a JOIN person p ON p.id=a.person_id JOIN position po ON po.id=a.position_id JOIN position_placement pl ON pl.position_id=a.position_id AND pl.tenant_id=a.tenant_id AND `+active("pl")+` JOIN org_unit u ON u.id=pl.org_unit_id LEFT JOIN position_reporting pr ON pr.position_id=a.position_id AND pr.tenant_id=a.tenant_id AND pr.relation_type='LINE' AND `+active("pr")+` WHERE a.tenant_id=? AND a.assignment_type='primary' AND `+active("a")+` ORDER BY u.code,po.code,p.id`, date, date, date, date, tenantID, date, date)
	if e != nil {
		return nil, e
	}
	byPos := map[string][]Row{}
	for _, p := range rows {
		id := rowString(p, "position_id")
		byPos[id] = append(byPos[id], p)
	}
	for _, p := range rows {
		supervisor := rowString(p, "supervisor_position_id")
		if supervisor == "" {
			p["reporting_status"] = "root"
			continue
		}
		managers := byPos[supervisor]
		switch len(managers) {
		case 0:
			p["reporting_status"] = "vacant_supervisor"
		case 1:
			p["manager_person_id"] = managers[0]["person_id"]
			p["manager_name"] = managers[0]["display_name"]
			p["reporting_status"] = "resolved"
		default:
			p["reporting_status"] = "ambiguous"
		}
	}
	visible := []Row{}
	for _, p := range rows {
		if unit != "" && rowString(p, "unit_code") != unit {
			continue
		}
		if team != "" && rowString(p, "position_code") != team {
			continue
		}
		visible = append(visible, p)
	}
	return map[string]any{"as_of": date, "total": len(visible), "people": visible, "units": units["units"], "edges": units["org_edges"], "positions": units["positions"], "view": "birdseye/department/team"}, nil
}

// Extra validation for illustration-only API filters.
var _ = strconv.Itoa
var _ = fmt.Sprintf

func boolInt(x bool) int {
	if x {
		return 1
	}
	return 0
}

func (s *Service) policyInt(key string, fallback int) int {
	r, e := s.one(`SELECT value FROM oa_policy WHERE tenant_id=? AND key=?`, tenantID, key)
	if e != nil || r == nil {
		return fallback
	}
	v, err := strconv.Atoi(rowString(r, "value"))
	if err != nil || v < 1 || v > 90 {
		return fallback
	}
	return v
}

// Any request for more information returns the case to its originator without
// manufacturing an approval. The reviewer note remains immutable in the step.
func (s *Service) resubmitV6(p *Principal, id string) (any, error) {
	c, e := s.one(`SELECT id,subject_id,opened_by,status FROM oa_case WHERE id=? AND tenant_id=?`, id, tenantID)
	if e != nil {
		return nil, e
	}
	if c == nil {
		return nil, fail(404, "Case not found")
	}
	if rowString(c, "status") != "NEEDS_INFO" {
		return nil, fail(409, "Case is not awaiting clarification")
	}
	if rowString(c, "opened_by") != p.PersonID {
		return nil, fail(403, "Only originator may resubmit")
	}
	if e := s.db.Exec("BEGIN IMMEDIATE"); e != nil {
		return nil, e
	}
	done := false
	defer func() {
		if !done {
			_ = s.db.Exec("ROLLBACK")
		}
	}()
	prev, e := s.one(`SELECT id,role FROM oa_case_step WHERE case_id=? AND status='NEED_INFO' ORDER BY seq DESC LIMIT 1`, id)
	if e != nil {
		return nil, e
	}
	if prev == nil {
		return nil, fail(409, "No clarification stage")
	}
	if e := s.db.Exec(`UPDATE oa_case_step SET status='WAITING' WHERE id=?`, prev["id"]); e != nil {
		return nil, e
	}
	if e := s.db.Exec(`UPDATE oa_case SET status='OPEN',stage=?,closed_at=NULL WHERE id=? AND status='NEEDS_INFO'`, prev["role"], id); e != nil {
		return nil, e
	}
	if e := auditV6(s, p.PersonID, "CASE_RESUBMIT", id, map[string]any{"stage": prev["role"]}); e != nil {
		return nil, e
	}
	if e := s.db.Exec("COMMIT"); e != nil {
		return nil, e
	}
	done = true
	return map[string]any{"id": id, "status": "OPEN", "stage": prev["role"]}, nil
}

// Seed case histories only into a completely empty synthetic tenant; this
// never updates user-created requests or any employment record.
func seedV6Cases(db SQLiteDB) error {
	row, e := first(db, `SELECT COUNT(*) AS n FROM oa_case WHERE tenant_id=?`, tenantID)
	if e != nil {
		return e
	}
	if intval(row["n"]) != 0 {
		return nil
	}
	count, e := first(db, `SELECT COUNT(*) AS n FROM person WHERE tenant_id=? AND synthetic=1`, tenantID)
	if e != nil {
		return e
	}
	if intval(count["n"]) != 1000 {
		return nil
	}
	if e := db.Exec("BEGIN IMMEDIATE"); e != nil {
		return e
	}
	done := false
	defer func() {
		if !done {
			_ = db.Exec("ROLLBACK")
		}
	}()
	for i := 12; i <= 19; i++ {
		kind := "BONUS"
		if i >= 18 {
			kind = "DISCIPLINE"
		}
		id := fmt.Sprintf("SYN-2026-%04d", i)
		subject := fmt.Sprintf("EMP-%04d", i)
		title := fmt.Sprintf("Sáng kiến cải tiến số %02d", i)
		description := "Cải tiến quản lý và an toàn vật tư trong kho; chứng từ demo theo mã KAIZEN-2026."
		tier, amount := kaizenReward(int64(i)*8_000_000, "FINANCIAL", 0, false)
		if kind == "DISCIPLINE" {
			title = "Hồ sơ sự việc giả lập"
			description = "Hồ sơ mô phỏng có chứng cứ, giải trình và thẩm quyền độc lập; không phải kết luận kỷ luật."
			tier = ""
			amount = 0
		}
		if e := db.Exec(`INSERT INTO oa_case(id,tenant_id,subject_id,opened_by,case_type,title,description,category,workdays,net_benefit,reward_estimate,reward_tier,stage,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'OPEN',?)`, id, tenantID, subject, "EMP-0009", kind, title, description, "FINANCIAL", 0, int64(i)*8_000_000, amount, tier, "HR", fmt.Sprintf("2026-10-%02dT09:00:00Z", i)); e != nil {
			return e
		}
		if e := db.Exec(`INSERT INTO oa_case_step(id,case_id,seq,role,assignee_id,status) VALUES(?,?,1,'HR','EMP-0008','WAITING')`, uid("STP"), id); e != nil {
			return e
		}
		if e := db.Exec(`INSERT INTO oa_case_step(id,case_id,seq,role,assignee_id,status) VALUES(?,?,2,'DIRECTOR','EMP-0001','BLOCKED')`, uid("STP"), id); e != nil {
			return e
		}
	}
	if e := db.Exec("COMMIT"); e != nil {
		return e
	}
	done = true
	return nil
}
