package main

import (
	"net/http"
	"strings"
	"time"
)

// Skill-only dispatch readiness is a preliminary check, NOT authorization to work.
func (s *Service) taskEligibility(r *http.Request) (any, error) {
	person := r.URL.Query().Get("person_id")
	taskCode := r.URL.Query().Get("task_code")
	date, e := argDate(r)
	if e != nil {
		return nil, e
	}
	if e = s.requirePerson(person); e != nil {
		return nil, e
	}
	task, e := s.one("SELECT id,name,code FROM work_task WHERE tenant_id=? AND code=?", tenantID, taskCode)
	if e != nil {
		return nil, e
	}
	if task == nil {
		return nil, fail(404, "Unknown task")
	}
	reqs, e := s.rows(`SELECT sk.code,sk.label,tr.min_level,COALESCE((SELECT MAX(ps.level) FROM person_skill ps WHERE ps.tenant_id=? AND ps.person_id=? AND ps.skill_id=sk.id AND ps.assessed_at<=? AND (ps.valid_until IS NULL OR ps.valid_until>=?)),0) AS highest_valid_level
 FROM task_requirement tr JOIN skill sk ON sk.id=tr.skill_id WHERE tr.task_id=? ORDER BY sk.code`, tenantID, person, date, date, task["id"])
	if e != nil {
		return nil, e
	}
	ok := len(reqs) > 0
	for _, x := range reqs {
		if intval(x["highest_valid_level"]) < intval(x["min_level"]) {
			ok = false
		}
	}
	status := "insufficient_skills"
	if ok {
		status = "skill_prerequisites_met_only"
	}
	return map[string]any{"person_id": person, "task_code": taskCode, "task_name": task["name"], "as_of": date, "requirements": reqs, "status": status, "authorized_to_work": false, "note": "Skill check only: verify licensing, relevant training, fitness, shift assignment, equipment and permits separately."}, nil
}
func (s *Service) hireDemo(name, positionCode, date string) (any, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 4 || len([]rune(name)) > 120 {
		return nil, fail(422, "Name must be 4..120 characters")
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	if date < time.Now().AddDate(-3, 0, 0).Format("2006-01-02") {
		return nil, fail(422, "Demo hire date outside supported range")
	}
	return s.transaction(func() (any, error) {
		p, e := s.position(positionCode, date)
		if e != nil {
			return nil, e
		}
		if p == nil {
			return nil, fail(404, "Unknown/inactive position")
		}
		used, e := count(s, `SELECT COUNT(*) AS n FROM assignment WHERE tenant_id=? AND position_id=? AND `+active(""), tenantID, p["id"], date, date)
		if e != nil {
			return nil, e
		}
		if used >= intval(p["holder_limit"]) {
			return nil, fail(409, "Position has reached holder limit")
		}
		person := uid("EMP")
		employment := uid("EE")
		assignment := uid("ASS")
		cs := uid("CS")
		if e = s.db.Exec(`INSERT INTO person(id,tenant_id,display_name,nationality,synthetic) VALUES (?,?,?,'VN',1)`, person, tenantID, name); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO employment_episode(id,tenant_id,person_id,legal_entity_id,relation_type,valid_from) VALUES (?,?,?,?,'employee',?)`, employment, tenantID, person, p["legal_entity_id"], date); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at) VALUES (?,?,?,?,?,'published',?)`, cs, tenantID, "workforce", "Demo hire "+person, date, time.Now().UTC().Format(time.RFC3339)); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO assignment(id,tenant_id,person_id,employment_id,position_id,assignment_type,fte,valid_from,change_set_id) VALUES (?,?,?,?,?,'primary',1,?,?)`, assignment, tenantID, person, employment, p["id"], date, cs); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO hr_event(id,tenant_id,person_id,event_type,effective_date,status,change_set_id,details) VALUES (?,?,?,'hire',?,'published',?,'{"synthetic":true}')`, uid("EV"), tenantID, person, date, cs); e != nil {
			return nil, e
		}
		if e = s.audit("person.hire", person, map[string]any{"name": name, "position": positionCode, "effective_date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"person_id": person, "employment_id": employment, "assignment_id": assignment, "effective_date": date, "synthetic": true}, nil
	})
}
func (s *Service) endEmployment(person, date, reason string) (any, error) {
	if !in(reason, "resignation", "retirement", "dismissal", "contract_end", "other") {
		return nil, fail(422, "Unsupported separation reason")
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	return s.transaction(func() (any, error) {
		if e := s.requirePerson(person); e != nil {
			return nil, e
		}
		a, e := s.assignment(person, date)
		if e != nil {
			return nil, e
		}
		if a == nil {
			return nil, fail(409, "No active assignment")
		}
		if date <= rowString(a, "valid_from") {
			return nil, fail(409, "Separation date must follow assignment start")
		}
		future, e := s.one(`SELECT id FROM assignment WHERE tenant_id=? AND person_id=? AND valid_from>=?`, tenantID, person, date)
		if e != nil {
			return nil, e
		}
		if future != nil {
			return nil, fail(409, "Future assignment exists")
		}
		emp, e := s.one(`SELECT id,valid_from FROM employment_episode WHERE tenant_id=? AND id=? AND (valid_to IS NULL OR valid_to>?)`, tenantID, a["employment_id"], date)
		if e != nil {
			return nil, e
		}
		if emp == nil {
			return nil, fail(409, "No active employment")
		}
		cs := uid("CS")
		if e = s.db.Exec(`INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at) VALUES (?,?,?,?,?,'published',?)`, cs, tenantID, "workforce", "Demo exit "+person, date, time.Now().UTC().Format(time.RFC3339)); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`UPDATE assignment SET valid_to=? WHERE id=?`, date, a["id"]); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`UPDATE employment_episode SET valid_to=? WHERE id=?`, date, emp["id"]); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO hr_event(id,tenant_id,person_id,event_type,effective_date,status,change_set_id,details) VALUES (?,?,?,? ,?,'published',?,'{"synthetic":true}')`, uid("EV"), tenantID, person, reason, date, cs); e != nil {
			return nil, e
		}
		if e = s.audit("person.separation", person, map[string]any{"reason": reason, "effective_date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"person_id": person, "effective_date": date, "reason": reason, "change_set_id": cs}, nil
	})
}

func (s *Service) tasksList() (any, error) {
	rs, e := s.rows(`SELECT id,code,name FROM work_task WHERE tenant_id=? ORDER BY code`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": rs}, nil
}
