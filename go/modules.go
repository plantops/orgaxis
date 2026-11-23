package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Demo modules are deliberately backed by relational SQLite data, not mock JSON.
// The seeded records are synthetic and no statutory entitlement decision is computed.
func enhanceDB(db SQLiteDB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS workflow_request (
          id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
          subject_person_id TEXT NOT NULL REFERENCES person(id),
          requester_id TEXT NOT NULL REFERENCES person(id), approver_id TEXT REFERENCES person(id),
          request_type TEXT NOT NULL, summary TEXT NOT NULL, requested_on TEXT NOT NULL,
          effective_date TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN ('pending','approved','rejected','unresolved')),
          decided_at TEXT, decision_note TEXT, routing_policy TEXT NOT NULL DEFAULT 'LINE_MANAGER',
          CHECK(length(summary) BETWEEN 3 AND 300))`,
		`CREATE INDEX IF NOT EXISTS idx_wf_status ON workflow_request(tenant_id,status,requested_on)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_person ON person_skill(tenant_id,person_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_date ON training_record(tenant_id,completed_on)`,
		`CREATE INDEX IF NOT EXISTS idx_kpi_period ON kpi_result(tenant_id,period)`,
		`CREATE INDEX IF NOT EXISTS idx_absence_status ON absence_case(tenant_id,approval_status)`,
	}
	for _, q := range schema {
		if err := db.Exec(q); err != nil {
			return fmt.Errorf("module migration: %w", err)
		}
	}
	r, err := first(db, "SELECT COUNT(*) AS n FROM person WHERE tenant_id=? AND synthetic=1", tenantID)
	if err != nil {
		return err
	}
	n, _ := strconv.Atoi(str(r["n"]))
	if n < 1000 {
		return nil
	} // Do not inject synthetic records into live or partial directories.
	skills, err := first(db, "SELECT COUNT(*) AS n FROM skill WHERE tenant_id=?", tenantID)
	if err != nil {
		return err
	}
	if intval(skills["n"]) > 0 {
		return nil
	}
	data, err := builtIn.ReadFile("assets/demo_additions.sql.gz")
	if err != nil {
		return fmt.Errorf("missing data backfill: %w", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 4096), 512*1024)
	if err = db.Exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = db.Exec("ROLLBACK")
		}
	}()
	for scan.Scan() {
		q := strings.TrimSpace(scan.Text())
		if q == "" {
			continue
		}
		if err = db.Exec(q); err != nil {
			return fmt.Errorf("seed backfill: %w", err)
		}
	}
	if err = scan.Err(); err != nil {
		return err
	}
	if err = db.Exec("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
func moduleList() []map[string]string {
	return []map[string]string{
		{"id": "organization", "label": "Tổ chức & cơ cấu", "mode": "operational"},
		{"id": "people", "label": "Hồ sơ nhân sự", "mode": "operational"},
		{"id": "skills", "label": "Năng lực & kỹ năng", "mode": "operational"},
		{"id": "training", "label": "Đào tạo & chứng nhận", "mode": "operational"},
		{"id": "performance", "label": "KPI & hiệu suất", "mode": "operational"},
		{"id": "absences", "label": "Nghỉ phép & vắng mặt", "mode": "operational"},
		{"id": "recognition", "label": "Khen thưởng", "mode": "operational"},
		{"id": "discipline", "label": "Kỷ luật & hồ sơ vụ việc", "mode": "record_only"},
		{"id": "occupational", "label": "Lịch sử nghề & điều kiện lao động", "mode": "record_only"},
		{"id": "workflow", "label": "Workflow & tuyến duyệt", "mode": "operational"},
		{"id": "payroll", "label": "Lương, BHXH, thuế", "mode": "record_only"},
		{"id": "immigration", "label": "Chuyên gia nước ngoài", "mode": "record_only"},
	}
}
func likeText(x string) string { return "%" + strings.ToLower(strings.TrimSpace(x)) + "%" }
func count(s *Service, sql string, args ...any) (int, error) {
	r, e := s.one(sql, args...)
	if e != nil {
		return 0, e
	}
	return intval(r["n"]), nil
}
func periodOrCurrent(v string) (string, error) {
	if v == "" {
		return "2026-10", nil
	}
	if ok, _ := regexp.MatchString(`^20\d{2}-(0[1-9]|1[0-2])$`, v); !ok {
		return "", fail(422, "Period must be YYYY-MM")
	}
	return v, nil
}
func (s *Service) dashboard(date string) (any, error) {
	totals := map[string]int{}
	for _, t := range []string{"person", "org_unit", "position", "person_skill", "training_record", "kpi_result", "recognition_case", "discipline_case", "absence_case", "workflow_request"} {
		n, e := count(s, "SELECT COUNT(*) AS n FROM "+t+" WHERE tenant_id=?", tenantID)
		if e != nil {
			return nil, e
		}
		totals[t] = n
	}
	activePersons, e := count(s, "SELECT COUNT(DISTINCT person_id) AS n FROM assignment WHERE tenant_id=? AND assignment_type='primary' AND "+active(""), tenantID, date, date)
	if e != nil {
		return nil, e
	}
	due := date
	if due < "2026-01-01" {
		due = "2026-01-01"
	}
	soon, _ := time.Parse("2006-01-02", due)
	dueEnd := soon.AddDate(0, 0, 45).Format("2006-01-02")
	expiring, e := count(s, `SELECT COUNT(*) AS n FROM training_record WHERE tenant_id=? AND valid_until>=? AND valid_until<=? AND result='passed'`, tenantID, date, dueEnd)
	if e != nil {
		return nil, e
	}
	pending, e := count(s, "SELECT COUNT(*) AS n FROM workflow_request WHERE tenant_id=? AND status='pending'", tenantID)
	if e != nil {
		return nil, e
	}
	vacancies, e := s.rows(`SELECT p.code,p.title,p.holder_limit,COALESCE(COUNT(a.id),0) AS staffed FROM position p
 JOIN position_placement pl ON pl.position_id=p.id AND `+active("pl")+`
 LEFT JOIN assignment a ON a.position_id=p.id AND a.assignment_type IN ('primary','acting') AND `+active("a")+`
 WHERE p.tenant_id=? AND p.created_at<=? GROUP BY p.id HAVING staffed<p.holder_limit ORDER BY p.holder_limit-staffed DESC LIMIT 8`, date, date, date, date, tenantID, date)
	if e != nil {
		return nil, e
	}
	acts, e := s.rows(`SELECT event_name,target,happened_at FROM audit_log WHERE tenant_id=? ORDER BY id DESC LIMIT 12`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"as_of": date, "totals": totals, "active_people": activePersons, "training_expiring_45d": expiring, "workflow_pending": pending, "vacancies": vacancies, "recent_activity": acts, "modules": moduleList()}, nil
}
func (s *Service) peopleSearch(r *http.Request) (any, error) {
	date, e := argDate(r)
	if e != nil {
		return nil, e
	}
	limit, e := takeInt(r, "limit", 50, 1, 1000)
	if e != nil {
		return nil, e
	}
	offset, e := takeInt(r, "offset", 0, 0, 1000000)
	if e != nil {
		return nil, e
	}
	q := likeText(r.URL.Query().Get("q"))
	unit := r.URL.Query().Get("unit")
	filter := ` WHERE pe.tenant_id=? AND (lower(pe.display_name) LIKE ? OR lower(pe.id) LIKE ?) AND (?='' OR u.code=?)`
	join := ` FROM person pe LEFT JOIN assignment a ON a.person_id=pe.id AND a.assignment_type='primary' AND ` + active("a") + `
 LEFT JOIN position p ON p.id=a.position_id
 LEFT JOIN position_placement pl ON pl.position_id=p.id AND ` + active("pl") + `
 LEFT JOIN org_unit u ON u.id=pl.org_unit_id`
	args := []any{date, date, date, date, tenantID, q, q, unit, unit}
	n, e := count(s, "SELECT COUNT(DISTINCT pe.id) AS n"+join+filter, args...)
	if e != nil {
		return nil, e
	}
	items, e := s.rows(`SELECT pe.id,pe.display_name,pe.nationality,p.code AS position_code,p.title AS position_title,u.code AS unit_code,u.name AS unit_name,a.valid_from AS assigned_from`+join+filter+` ORDER BY pe.id LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": items, "total": n, "limit": limit, "offset": offset, "as_of": date}, nil
}
func (s *Service) personDetail(person, date string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	profile, e := s.one("SELECT id,display_name,nationality,synthetic FROM person WHERE id=? AND tenant_id=?", person, tenantID)
	if e != nil {
		return nil, e
	}
	assignment, e := s.rows(`SELECT a.valid_from,a.valid_to,a.assignment_type,p.title,p.code AS position_code,u.name AS unit_name,u.code AS unit_code
 FROM assignment a JOIN position p ON p.id=a.position_id
 JOIN position_placement pl ON pl.position_id=p.id AND pl.valid_from<=a.valid_from AND (pl.valid_to IS NULL OR pl.valid_to>a.valid_from)
 JOIN org_unit u ON u.id=pl.org_unit_id WHERE a.tenant_id=? AND a.person_id=? AND a.valid_from<=? ORDER BY a.valid_from DESC LIMIT 12`, tenantID, person, date)
	if e != nil {
		return nil, e
	}
	mgr, e := s.manager(person, date)
	if e != nil {
		return nil, e
	}
	skills, e := s.rows(`SELECT ps.id,sk.code,sk.label,ps.level,ps.assessed_at,ps.valid_until FROM person_skill ps JOIN skill sk ON sk.id=ps.skill_id WHERE ps.tenant_id=? AND ps.person_id=? ORDER BY ps.assessed_at DESC LIMIT 20`, tenantID, person)
	if e != nil {
		return nil, e
	}
	training, e := s.rows(`SELECT tp.name,tr.completed_on,tr.result,tr.valid_until FROM training_record tr JOIN training_program tp ON tp.id=tr.program_id WHERE tr.tenant_id=? AND tr.person_id=? ORDER BY tr.completed_on DESC LIMIT 16`, tenantID, person)
	if e != nil {
		return nil, e
	}
	results, e := s.rows(`SELECT period,metric_code,actual,target FROM kpi_result WHERE tenant_id=? AND person_id=? ORDER BY period DESC,metric_code LIMIT 20`, tenantID, person)
	if e != nil {
		return nil, e
	}
	absences, e := s.rows(`SELECT id,absence_type,valid_from,valid_to,approval_status FROM absence_case WHERE employment_id IN (SELECT id FROM employment_episode WHERE person_id=? AND tenant_id=?) ORDER BY valid_from DESC LIMIT 12`, person, tenantID)
	if e != nil {
		return nil, e
	}
	awards, e := s.rows(`SELECT case_type,effective_date,status FROM recognition_case WHERE tenant_id=? AND person_id=? ORDER BY effective_date DESC LIMIT 12`, tenantID, person)
	if e != nil {
		return nil, e
	}
	occupation, e := s.rows(`SELECT employer_label,occupation_label,condition_category,valid_from,valid_to,classification_rule_ref FROM occupation_period WHERE tenant_id=? AND person_id=? ORDER BY valid_from DESC`, tenantID, person)
	if e != nil {
		return nil, e
	}
	return map[string]any{"person": profile, "manager": mgr, "assignments": assignment, "skills": skills, "training": training, "kpi": results, "absences": absences, "recognitions": awards, "occupational": occupation, "as_of": date}, nil
}
func (s *Service) skillsList() (any, error) {
	rows, e := s.rows(`SELECT sk.id,sk.code,sk.label,COUNT(ps.id) AS holders, SUM(CASE WHEN ps.level>=3 THEN 1 ELSE 0 END) AS proficient FROM skill sk LEFT JOIN person_skill ps ON ps.skill_id=sk.id WHERE sk.tenant_id=? GROUP BY sk.id ORDER BY holders DESC,sk.code`, tenantID)
	if e != nil {
		return nil, e
	}
	multi, e := count(s, `SELECT COUNT(*) AS n FROM (SELECT person_id FROM person_skill WHERE tenant_id=? GROUP BY person_id HAVING COUNT(DISTINCT skill_id)>=2)`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rows, "people_multiskill": multi}, nil
}
func (s *Service) skillHolders(r *http.Request) (any, error) {
	skill := r.URL.Query().Get("skill")
	q := likeText(r.URL.Query().Get("q"))
	limit, e := takeInt(r, "limit", 60, 1, 1000)
	if e != nil {
		return nil, e
	}
	items, e := s.rows(`SELECT pe.id AS person_id,pe.display_name,sk.label,sk.code,ps.level,ps.assessed_at,ps.valid_until
 FROM person_skill ps JOIN person pe ON pe.id=ps.person_id JOIN skill sk ON sk.id=ps.skill_id
 WHERE ps.tenant_id=? AND (?='' OR sk.code=?) AND (lower(pe.display_name) LIKE ? OR lower(pe.id) LIKE ?)
 ORDER BY ps.level DESC,pe.id LIMIT ?`, tenantID, skill, skill, q, q, limit)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": items}, nil
}
func (s *Service) skillAward(person, code string, level int, date, expires string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if level < 1 || level > 4 {
		return nil, fail(422, "Skill level must be 1..4")
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	if expires != "" {
		if _, e := requireDate(expires); e != nil {
			return nil, e
		}
		if expires < date {
			return nil, fail(422, "Expiration precedes assessment")
		}
	}
	sk, e := s.one("SELECT id FROM skill WHERE code=? AND tenant_id=?", code, tenantID)
	if e != nil {
		return nil, e
	}
	if sk == nil {
		return nil, fail(404, "Unknown skill")
	}
	return s.transaction(func() (any, error) {
		id := uid("PS")
		if e := s.db.Exec(`INSERT INTO person_skill(id,tenant_id,person_id,skill_id,level,assessed_at,valid_until) VALUES (?,?,?,?,?,?,?)`, id, tenantID, person, sk["id"], level, date, nullableTime(expires)); e != nil {
			return nil, e
		}
		if e := s.audit("skill.assess", person, map[string]any{"skill": code, "level": level, "date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "person_id": person, "skill": code, "level": level, "assessed_at": date}, nil
	})
}
func (s *Service) trainingPrograms() (any, error) {
	rs, e := s.rows(`SELECT p.id,p.name,p.regulatory_basis,p.version,COUNT(r.id) AS attendees FROM training_program p LEFT JOIN training_record r ON r.program_id=p.id WHERE p.tenant_id=? GROUP BY p.id ORDER BY p.name`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rs}, nil
}
func (s *Service) trainingRecords(r *http.Request) (any, error) {
	q := likeText(r.URL.Query().Get("q"))
	program := r.URL.Query().Get("program")
	limit, e := takeInt(r, "limit", 60, 1, 1000)
	if e != nil {
		return nil, e
	}
	rows, e := s.rows(`SELECT tr.id,tr.person_id,pe.display_name,p.name AS program,tr.result,tr.completed_on,tr.valid_until
 FROM training_record tr JOIN person pe ON pe.id=tr.person_id JOIN training_program p ON p.id=tr.program_id
 WHERE tr.tenant_id=? AND (?='' OR p.id=?) AND (lower(pe.display_name) LIKE ? OR lower(pe.id) LIKE ?)
 ORDER BY tr.completed_on DESC,tr.id LIMIT ?`, tenantID, program, program, q, q, limit)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rows}, nil
}
func (s *Service) trainingAdd(person, program, date, result, expires string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if result != "passed" && result != "attended" && result != "failed" {
		return nil, fail(422, "Invalid training result")
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	if expires != "" {
		if _, e := requireDate(expires); e != nil {
			return nil, e
		}
		if expires < date {
			return nil, fail(422, "Expiry before completion")
		}
	}
	p, e := s.one("SELECT id FROM training_program WHERE id=? AND tenant_id=?", program, tenantID)
	if e != nil {
		return nil, e
	}
	if p == nil {
		return nil, fail(404, "Unknown training program")
	}
	return s.transaction(func() (any, error) {
		id := uid("TR")
		if e := s.db.Exec(`INSERT INTO training_record(id,tenant_id,person_id,program_id,completed_on,result,valid_until) VALUES (?,?,?,?,?,?,?)`, id, tenantID, person, program, date, result, nullableTime(expires)); e != nil {
			return nil, e
		}
		if e := s.audit("training.record", person, map[string]any{"program": program, "date": date, "result": result}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "person_id": person, "result": result}, nil
	})
}
func (s *Service) performance(period string) (any, error) {
	sums, e := s.rows(`SELECT metric_code,ROUND(AVG(actual),1) AS average_actual,ROUND(AVG(target),1) AS average_target,COUNT(*) AS records,
 SUM(CASE WHEN actual>=target THEN 1 ELSE 0 END) AS reached FROM kpi_result WHERE tenant_id=? AND period=? GROUP BY metric_code ORDER BY metric_code`, tenantID, period)
	if e != nil {
		return nil, e
	}
	results, e := s.rows(`SELECT k.person_id,p.display_name,k.metric_code,k.actual,k.target,k.period FROM kpi_result k JOIN person p ON p.id=k.person_id WHERE k.tenant_id=? AND k.period=? ORDER BY k.actual*1.0/NULLIF(k.target,0) DESC LIMIT 80`, tenantID, period)
	if e != nil {
		return nil, e
	}
	return map[string]any{"period": period, "metrics": sums, "results": results}, nil
}
func (s *Service) kpiUpdate(person, period, metric string, actual, target float64) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if actual < 0 || target <= 0 || target > 1e9 || actual > 1e9 {
		return nil, fail(422, "Invalid metric values")
	}
	if len(metric) < 2 || len(metric) > 32 {
		return nil, fail(422, "Invalid metric code")
	}
	return s.transaction(func() (any, error) {
		old, e := s.one(`SELECT id,actual,target FROM kpi_result WHERE tenant_id=? AND person_id=? AND period=? AND metric_code=? ORDER BY id DESC LIMIT 1`, tenantID, person, period, metric)
		if e != nil {
			return nil, e
		}
		id := uid("KPI")
		if old != nil {
			id = rowString(old, "id")
			e = s.db.Exec(`UPDATE kpi_result SET actual=?,target=?,source_type='demo_manual' WHERE id=?`, actual, target, id)
		} else {
			e = s.db.Exec(`INSERT INTO kpi_result(id,tenant_id,person_id,period,metric_code,actual,target,source_type) VALUES (?,?,?,?,?,?,?,'demo_manual')`, id, tenantID, person, period, metric, actual, target)
		}
		if e != nil {
			return nil, e
		}
		if e = s.audit("kpi.record", person, map[string]any{"period": period, "metric": metric, "old": old, "actual": actual, "target": target}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "person_id": person, "period": period, "metric": metric, "actual": actual, "target": target}, nil
	})
}
func (s *Service) absences(r *http.Request) (any, error) {
	status := r.URL.Query().Get("status")
	limit, e := takeInt(r, "limit", 80, 1, 1000)
	if e != nil {
		return nil, e
	}
	rs, e := s.rows(`SELECT ac.id,ee.person_id,p.display_name,ac.absence_type,ac.valid_from,ac.valid_to,ac.approval_status
 FROM absence_case ac JOIN employment_episode ee ON ee.id=ac.employment_id JOIN person p ON p.id=ee.person_id
 WHERE ac.tenant_id=? AND (?='' OR ac.approval_status=?) ORDER BY ac.valid_from DESC LIMIT ?`, tenantID, status, status, limit)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rs}, nil
}
func (s *Service) absenceAdd(person, kind, start, end string) (any, error) {
	if _, e := requireDate(start); e != nil {
		return nil, e
	}
	if _, e := requireDate(end); e != nil {
		return nil, e
	}
	if end <= start {
		return nil, fail(422, "End must be later than start (exclusive)")
	}
	if !in(kind, "annual", "sick", "maternity", "unpaid", "other") {
		return nil, fail(422, "Unsupported absence type")
	}
	emp, e := s.one(`SELECT id FROM employment_episode WHERE tenant_id=? AND person_id=? AND valid_from<=? AND (valid_to IS NULL OR valid_to>?) ORDER BY valid_from DESC LIMIT 1`, tenantID, person, start, start)
	if e != nil {
		return nil, e
	}
	if emp == nil {
		return nil, fail(409, "No active employment at start")
	}
	return s.transaction(func() (any, error) {
		id := uid("ABS")
		if e := s.db.Exec(`INSERT INTO absence_case VALUES (?,?,?,?,?,?,?,?)`, id, tenantID, emp["id"], kind, start, end, "pending", "SYNTHETIC-UI"); e != nil {
			return nil, e
		}
		if e := s.audit("absence.create", person, map[string]any{"kind": kind, "from": start, "to": end}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": "pending", "person_id": person}, nil
	})
}
func (s *Service) absenceDecision(id, status string) (any, error) {
	if !in(status, "approved", "rejected") {
		return nil, fail(422, "Only approved/rejected supported")
	}
	return s.transaction(func() (any, error) {
		old, e := s.one(`SELECT approval_status FROM absence_case WHERE id=? AND tenant_id=?`, id, tenantID)
		if e != nil {
			return nil, e
		}
		if old == nil {
			return nil, fail(404, "Unknown case")
		}
		if old["approval_status"] != "pending" {
			return nil, fail(409, "Case is not pending")
		}
		if e := s.db.Exec(`UPDATE absence_case SET approval_status=? WHERE id=?`, status, id); e != nil {
			return nil, e
		}
		if e := s.audit("absence.decision", id, map[string]any{"status": status}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": status}, nil
	})
}
func (s *Service) recognitionList(kind string) (any, error) {
	if kind == "recognition" {
		rs, e := s.rows(`SELECT c.id,c.person_id,p.display_name,c.case_type,c.effective_date,c.status FROM recognition_case c JOIN person p ON p.id=c.person_id WHERE c.tenant_id=? ORDER BY c.effective_date DESC LIMIT 120`, tenantID)
		return map[string]any{"items": rs}, e
	}
	rs, e := s.rows(`SELECT c.id,c.person_id,p.display_name,c.opened_on,c.case_status,c.decision_type FROM discipline_case c JOIN person p ON p.id=c.person_id WHERE c.tenant_id=? ORDER BY c.opened_on DESC LIMIT 120`, tenantID)
	return map[string]any{"items": rs}, e
}
func (s *Service) recognitionAdd(person, category, date string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	if len(category) < 3 || len(category) > 120 {
		return nil, fail(422, "Invalid category")
	}
	return s.transaction(func() (any, error) {
		id := uid("REW")
		if e := s.db.Exec(`INSERT INTO recognition_case(id,tenant_id,person_id,case_type,effective_date,status) VALUES (?,?,?,?,?,'proposed')`, id, tenantID, person, category, date); e != nil {
			return nil, e
		}
		if e := s.audit("recognition.propose", person, map[string]any{"category": category, "date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": "proposed"}, nil
	})
}
func (s *Service) disciplinaryOpen(person, date string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	return s.transaction(func() (any, error) {
		id := uid("CASE")
		if e := s.db.Exec(`INSERT INTO discipline_case(id,tenant_id,person_id,opened_on,case_status) VALUES (?,?,?,?,?)`, id, tenantID, person, date, "under_review"); e != nil {
			return nil, e
		}
		if e := s.audit("discipline.open", person, map[string]any{"date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": "under_review"}, nil
	})
}
func (s *Service) occupationList(r *http.Request) (any, error) {
	category := r.URL.Query().Get("category")
	limit, e := takeInt(r, "limit", 100, 1, 1000)
	if e != nil {
		return nil, e
	}
	items, e := s.rows(`SELECT o.id,o.person_id,p.display_name,o.employer_label,o.occupation_label,o.condition_category,o.valid_from,o.valid_to,o.classification_rule_ref
 FROM occupation_period o JOIN person p ON p.id=o.person_id WHERE o.tenant_id=? AND (?='' OR o.condition_category=?) ORDER BY o.valid_from DESC,o.id LIMIT ?`, tenantID, category, category, limit)
	if e != nil {
		return nil, e
	}
	grades, e := s.rows(`SELECT condition_category,COUNT(*) AS records FROM occupation_period WHERE tenant_id=? GROUP BY condition_category ORDER BY condition_category`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": items, "grades": grades, "verification": "Synthetic classifications; not legal entitlements"}, nil
}
func (s *Service) workflowList(r *http.Request) (any, error) {
	status := r.URL.Query().Get("status")
	limit, e := takeInt(r, "limit", 100, 1, 1000)
	if e != nil {
		return nil, e
	}
	rs, e := s.rows(`SELECT w.id,w.request_type,w.summary,w.subject_person_id,p.display_name AS subject_name,w.approver_id,ap.display_name AS approver_name,w.requested_on,w.effective_date,w.status,w.decided_at
 FROM workflow_request w JOIN person p ON p.id=w.subject_person_id LEFT JOIN person ap ON ap.id=w.approver_id
 WHERE w.tenant_id=? AND (?='' OR w.status=?) ORDER BY w.requested_on DESC,w.id LIMIT ?`, tenantID, status, status, limit)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rs}, nil
}
func (s *Service) workflowCreate(subject, kind, summary, date string) (any, error) {
	if e := s.requirePerson(subject); e != nil {
		return nil, e
	}
	if !in(kind, "leave", "travel", "purchase", "shift_assignment", "training", "other") {
		return nil, fail(422, "Unsupported workflow type")
	}
	if len(summary) < 3 || len(summary) > 300 {
		return nil, fail(422, "Summary must be 3..300 characters")
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	route, e := s.resolve(subject, date, "LINE_MANAGER", 1)
	if e != nil {
		return nil, e
	}
	status := "unresolved"
	var approver any
	if route["status"] == "resolved" {
		status = "pending"
		approver = route["assignee_id"]
	}
	return s.transaction(func() (any, error) {
		id := uid("WF")
		if e := s.db.Exec(`INSERT INTO workflow_request(id,tenant_id,subject_person_id,requester_id,approver_id,request_type,summary,requested_on,effective_date,status) VALUES (?,?,?,?,?,?,?,?,?,?)`, id, tenantID, subject, subject, approver, kind, summary, time.Now().Format("2006-01-02"), date, status); e != nil {
			return nil, e
		}
		if e := s.audit("workflow.create", id, map[string]any{"subject": subject, "approver": approver, "date": date, "kind": kind}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": status, "approver_id": approver, "effective_date": date}, nil
	})
}
func (s *Service) workflowDecision(id, status, note string) (any, error) {
	if !in(status, "approved", "rejected") {
		return nil, fail(422, "Only approved or rejected supported")
	}
	if len(note) > 500 {
		return nil, fail(422, "Decision note too long")
	}
	return s.transaction(func() (any, error) {
		old, e := s.one(`SELECT status,approver_id FROM workflow_request WHERE id=? AND tenant_id=?`, id, tenantID)
		if e != nil {
			return nil, e
		}
		if old == nil {
			return nil, fail(404, "Unknown workflow")
		}
		if old["status"] != "pending" {
			return nil, fail(409, "Workflow not pending")
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if e = s.db.Exec(`UPDATE workflow_request SET status=?,decided_at=?,decision_note=? WHERE id=?`, status, now, note, id); e != nil {
			return nil, e
		}
		if e := s.audit("workflow.decision", id, map[string]any{"status": status, "approver": old["approver_id"], "note": note}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": status, "decided_at": now}, nil
	})
}
func in(s string, choices ...string) bool {
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}

func (s *Service) moduleAPI(r *http.Request) (any, error) {
	p := r.URL.Path
	m := r.Method
	switch {
	case p == "/v1/workforce/tasks" && m == "GET":
		return s.tasksList()
	case p == "/v1/workforce/eligibility" && m == "GET":
		return s.taskEligibility(r)
	case p == "/v1/people" && m == "POST":
		var x struct {
			Name          string `json:"name"`
			PositionCode  string `json:"position_code"`
			EffectiveDate string `json:"effective_date"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.hireDemo(x.Name, x.PositionCode, x.EffectiveDate)
	case strings.HasPrefix(p, "/v1/people/") && strings.HasSuffix(p, "/separation") && m == "POST":
		var x struct {
			EffectiveDate string `json:"effective_date"`
			Reason        string `json:"reason"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.endEmployment(strings.TrimSuffix(strings.TrimPrefix(p, "/v1/people/"), "/separation"), x.EffectiveDate, x.Reason)
	case p == "/v1/payroll/workbench" && m == "GET":
		return s.payrollWorkbench(r)
	case p == "/v1/payroll/simulate" && m == "GET":
		return s.payrollTaxSimulation(r)
	case strings.HasPrefix(p, "/v1/payroll/payslip/") && m == "GET":
		return s.payrollPayslip(strings.TrimPrefix(p, "/v1/payroll/payslip/"), r.URL.Query().Get("period"))
	case p == "/v1/payroll/summary" && m == "GET":
		return s.payrollSummary(r)
	case p == "/v1/benefits/claims" && m == "GET":
		return s.benefitList()
	case p == "/v1/benefits/claims" && m == "POST":
		var x struct {
			PersonID string `json:"person_id"`
			Type     string `json:"type"`
			Date     string `json:"date"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.benefitAdd(x.PersonID, x.Type, x.Date)
	case p == "/v1/immigration/summary" && m == "GET":
		return s.immigrationSummary(r)
	case p == "/v1/modules" && m == "GET":
		return map[string]any{"items": moduleList()}, nil
	case p == "/v1/dashboard" && m == "GET":
		d, e := argDate(r)
		if e != nil {
			return nil, e
		}
		return s.dashboard(d)
	case p == "/v1/people/search" && m == "GET":
		return s.peopleSearch(r)
	case strings.HasPrefix(p, "/v1/people/detail/") && m == "GET":
		d, e := argDate(r)
		if e != nil {
			return nil, e
		}
		return s.personDetail(strings.TrimPrefix(p, "/v1/people/detail/"), d)
	case p == "/v1/skills/catalog" && m == "GET":
		return s.skillsList()
	case p == "/v1/skills/holders" && m == "GET":
		return s.skillHolders(r)
	case p == "/v1/skills/assess" && m == "POST":
		var x struct {
			PersonID   string `json:"person_id"`
			SkillCode  string `json:"skill_code"`
			Level      int    `json:"level"`
			AssessedAt string `json:"assessed_at"`
			ValidUntil string `json:"valid_until"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.skillAward(x.PersonID, x.SkillCode, x.Level, x.AssessedAt, x.ValidUntil)
	case p == "/v1/training/programs" && m == "GET":
		return s.trainingPrograms()
	case p == "/v1/training/records" && m == "GET":
		return s.trainingRecords(r)
	case p == "/v1/training/records" && m == "POST":
		var x struct {
			PersonID    string `json:"person_id"`
			ProgramID   string `json:"program_id"`
			CompletedOn string `json:"completed_on"`
			Result      string `json:"result"`
			ValidUntil  string `json:"valid_until"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.trainingAdd(x.PersonID, x.ProgramID, x.CompletedOn, x.Result, x.ValidUntil)
	case p == "/v1/performance" && m == "GET":
		period, e := periodOrCurrent(r.URL.Query().Get("period"))
		if e != nil {
			return nil, e
		}
		return s.performance(period)
	case p == "/v1/performance" && m == "POST":
		var x struct {
			PersonID string  `json:"person_id"`
			Period   string  `json:"period"`
			Metric   string  `json:"metric_code"`
			Actual   float64 `json:"actual"`
			Target   float64 `json:"target"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		period, e := periodOrCurrent(x.Period)
		if e != nil {
			return nil, e
		}
		return s.kpiUpdate(x.PersonID, period, x.Metric, x.Actual, x.Target)
	case p == "/v1/absences" && m == "GET":
		return s.absences(r)
	case p == "/v1/absences" && m == "POST":
		var x struct {
			PersonID string `json:"person_id"`
			Type     string `json:"type"`
			Start    string `json:"start"`
			End      string `json:"end"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.absenceAdd(x.PersonID, x.Type, x.Start, x.End)
	case strings.HasPrefix(p, "/v1/absences/") && strings.HasSuffix(p, "/decision") && m == "POST":
		var x struct {
			Status string `json:"status"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.absenceDecision(strings.TrimSuffix(strings.TrimPrefix(p, "/v1/absences/"), "/decision"), x.Status)
	case p == "/v1/recognitions" && m == "GET":
		return s.recognitionList("recognition")
	case p == "/v1/recognitions" && m == "POST":
		var x struct {
			PersonID string `json:"person_id"`
			Category string `json:"category"`
			Date     string `json:"date"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.recognitionAdd(x.PersonID, x.Category, x.Date)
	case p == "/v1/discipline" && m == "GET":
		return s.recognitionList("discipline")
	case p == "/v1/discipline" && m == "POST":
		var x struct {
			PersonID string `json:"person_id"`
			Date     string `json:"date"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.disciplinaryOpen(x.PersonID, x.Date)
	case p == "/v1/occupational" && m == "GET":
		return s.occupationList(r)
	case p == "/v1/workflow/requests" && m == "GET":
		return s.workflowList(r)
	case p == "/v1/workflow/requests" && m == "POST":
		var x struct {
			SubjectID     string `json:"subject_id"`
			Type          string `json:"type"`
			Summary       string `json:"summary"`
			EffectiveDate string `json:"effective_date"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.workflowCreate(x.SubjectID, x.Type, x.Summary, x.EffectiveDate)
	case strings.HasPrefix(p, "/v1/workflow/requests/") && strings.HasSuffix(p, "/decision") && m == "POST":
		var x struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if e := bodyJSON(r, &x); e != nil {
			return nil, e
		}
		return s.workflowDecision(strings.TrimSuffix(strings.TrimPrefix(p, "/v1/workflow/requests/"), "/decision"), x.Status, x.Note)
	}
	return nil, fail(404, "Unknown endpoint")
}
