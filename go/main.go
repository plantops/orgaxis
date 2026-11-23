package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed assets/*.html assets/*.js assets/*.css assets/seed.sqlite3 assets/*.gz assets/*.ico assets/avatars/*.webp assets/illustrations/*.webp
var builtIn embed.FS

const tenantID = "tenant-cfc-pilot"
const todayFallback = "2026-11-18"

type Service struct {
	db         SQLiteDB
	mu         sync.Mutex
	key        string
	port       int
	logfile    *RotatingLog
	started    time.Time
	secureDemo bool
}
type APIError struct {
	Status  int
	Message string
}

func (e APIError) Error() string        { return e.Message }
func fail(status int, msg string) error { return APIError{status, msg} }
func check(err error) {
	if err != nil {
		panic(err)
	}
}
func requireDate(s string) (string, error) {
	if s == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	d, e := time.Parse("2006-01-02", s)
	if e != nil || d.Format("2006-01-02") != s {
		return "", fail(422, "Date must be YYYY-MM-DD")
	}
	return s, nil
}
func active(alias string) string {
	if alias != "" {
		alias += "."
	}
	return alias + "valid_from <= ? AND (" + alias + "valid_to IS NULL OR " + alias + "valid_to > ?)"
}
func uid(prefix string) string {
	b := make([]byte, 12)
	_, e := rand.Read(b)
	check(e)
	return prefix + "-" + hex.EncodeToString(b)
}
func takeInt(r *http.Request, key string, def, min, max int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def, nil
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < min || n > max {
		return 0, fail(422, fmt.Sprintf("%s must be %d..%d", key, min, max))
	}
	return n, nil
}
func argDate(r *http.Request) (string, error) { return requireDate(r.URL.Query().Get("as_of")) }
func response(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(obj)
}
func handleError(w http.ResponseWriter, e error) {
	var a APIError
	if errors.As(e, &a) {
		response(w, a.Status, map[string]any{"detail": a.Message})
		return
	}
	log.Printf("internal error: %v", e)
	response(w, 500, map[string]any{"detail": "Internal server error"})
}
func (s *Service) api(w http.ResponseWriter, r *http.Request, fn func() (any, error)) {
	if s.secureDemo {
		if r.Context().Value(lanAuthenticatedKey{}) != true {
			s.mu.Lock()
			p, e := s.principal(r)
			s.mu.Unlock()
			if e != nil {
				handleError(w, e)
				return
			}
			if !role(p, "ADMIN") {
				response(w, 403, map[string]any{"detail": "Legacy v1 API restricted to ADMIN; use scoped /v2 endpoints"})
				return
			}
			if r.Method != "GET" && r.Header.Get("X-CSRF-Token") != p.CSRF {
				response(w, 403, map[string]any{"detail": "CSRF required"})
				return
			}
		}
	} else if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(s.key)) != 1 {
		response(w, 401, map[string]any{"detail": "Invalid API key"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, e := fn()
	if e != nil {
		handleError(w, e)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	response(w, status, data)
}
func bodyJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return fail(422, "Invalid JSON request")
	}
	var trailing any
	if e := dec.Decode(&trailing); e != io.EOF {
		return fail(422, "Only one JSON object allowed")
	}
	return nil
}
func (s *Service) one(q string, args ...any) (Row, error)    { return first(s.db, q, args...) }
func (s *Service) rows(q string, args ...any) ([]Row, error) { return s.db.Query(q, args...) }
func (s *Service) requirePerson(person string) error {
	p, e := s.one("SELECT id FROM person WHERE tenant_id=? AND id=?", tenantID, person)
	if e != nil {
		return e
	}
	if p == nil {
		return fail(404, "Unknown person")
	}
	return nil
}
func (s *Service) assignment(person, date string) (Row, error) {
	return s.one(`SELECT a.id,a.position_id,a.employment_id,a.valid_from,a.valid_to,p.code AS position_code,p.title
FROM assignment a JOIN position p ON p.id=a.position_id
WHERE a.tenant_id=? AND a.person_id=? AND a.assignment_type='primary'
AND `+active("a")+` ORDER BY a.valid_from DESC LIMIT 1`, tenantID, person, date, date)
}
func (s *Service) manager(person, date string) (map[string]any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	assignment, e := s.assignment(person, date)
	if e != nil {
		return nil, e
	}
	result := map[string]any{"person_id": person, "as_of": date, "manager": nil}
	if assignment == nil {
		result["status"] = "unassigned"
		return result, nil
	}
	result["position_code"] = assignment["position_code"]
	links, e := s.rows(`SELECT r.supervisor_position_id,p.code AS supervisor_code FROM position_reporting r JOIN position p ON p.id=r.supervisor_position_id WHERE r.tenant_id=? AND r.position_id=? AND r.relation_type='LINE' AND `+active("r"), tenantID, assignment["position_id"], date, date)
	if e != nil {
		return nil, e
	}
	if len(links) > 1 {
		result["status"] = "ambiguous_reporting"
		return result, nil
	}
	if len(links) == 0 {
		result["status"] = "root"
		return result, nil
	}
	parent := links[0]
	result["supervisor_position_id"] = parent["supervisor_position_id"]
	result["supervisor_position_code"] = parent["supervisor_code"]
	staff, e := s.rows(`SELECT a.person_id,p.display_name FROM assignment a JOIN person p ON p.id=a.person_id WHERE a.tenant_id=? AND a.position_id=? AND a.assignment_type IN ('primary','acting') AND `+active("a"), tenantID, parent["supervisor_position_id"], date, date)
	if e != nil {
		return nil, e
	}
	switch len(staff) {
	case 0:
		result["status"] = "vacant_supervisor"
	case 1:
		result["status"] = "resolved"
		result["manager"] = staff[0]
	default:
		result["status"] = "ambiguous_supervisor"
	}
	return result, nil
}
func (s *Service) chain(person, date string, maxLevels int) (map[string]any, error) {
	chain := make([]map[string]any, 0)
	seen := map[string]bool{person: true}
	curr := person
	for i := 0; i < maxLevels; i++ {
		m, e := s.manager(curr, date)
		if e != nil {
			return nil, e
		}
		if m["status"] != "resolved" {
			return map[string]any{"person_id": person, "as_of": date, "status": m["status"], "chain": chain, "terminal": m}, nil
		}
		mgr := m["manager"].(Row)
		next := rowString(mgr, "person_id")
		if seen[next] {
			return nil, fail(409, "Cycle in reporting chain")
		}
		seen[next] = true
		chain = append(chain, map[string]any{"level": i + 1, "person_id": next, "display_name": mgr["display_name"], "position_code": m["supervisor_position_code"]})
		curr = next
	}
	return map[string]any{"person_id": person, "as_of": date, "status": "limit_reached", "chain": chain}, nil
}
func (s *Service) blankOrg(date string) (map[string]any, error) {
	units, e := s.rows(`SELECT id,code,name,unit_type,legal_entity_id FROM org_unit WHERE tenant_id=? AND created_at<=? ORDER BY code`, tenantID, date)
	if e != nil {
		return nil, e
	}
	edges, e := s.rows(`SELECT parent_id,child_id,relation_type FROM org_unit_relation WHERE tenant_id=? AND `+active("")+` ORDER BY child_id`, tenantID, date, date)
	if e != nil {
		return nil, e
	}
	positions, e := s.rows(`SELECT p.id,p.code,p.title,p.holder_limit,pl.org_unit_id,r.supervisor_position_id
FROM position p JOIN position_placement pl ON pl.position_id=p.id AND pl.tenant_id=p.tenant_id
LEFT JOIN position_reporting r ON r.position_id=p.id AND r.tenant_id=p.tenant_id AND r.relation_type='LINE' AND `+active("r")+`
WHERE p.tenant_id=? AND p.created_at<=? AND `+active("pl")+` ORDER BY p.code`, date, date, tenantID, date, date, date)
	if e != nil {
		return nil, e
	}
	return map[string]any{"as_of": date, "units": units, "org_edges": edges, "positions": positions, "unit_count": len(units), "position_count": len(positions)}, nil
}
func (s *Service) occupied(date string, limit, offset int) (map[string]any, error) {
	staff, e := s.rows(`SELECT a.person_id,per.display_name,p.code AS position_code,a.assignment_type,pl.org_unit_id
FROM assignment a JOIN person per ON per.id=a.person_id JOIN position p ON p.id=a.position_id
LEFT JOIN position_placement pl ON pl.position_id=p.id AND `+active("pl")+`
WHERE a.tenant_id=? AND `+active("a")+` ORDER BY a.person_id LIMIT ? OFFSET ?`, date, date, tenantID, date, date, limit, offset)
	if e != nil {
		return nil, e
	}
	cnt, e := s.one(`SELECT COUNT(*) AS n FROM assignment a WHERE a.tenant_id=? AND `+active("a"), tenantID, date, date)
	if e != nil {
		return nil, e
	}
	return map[string]any{"as_of": date, "limit": limit, "offset": offset, "total": intval(cnt["n"]), "assignments": staff}, nil
}
func (s *Service) directReports(person, date string, limit, offset int) (map[string]any, error) {
	a, e := s.assignment(person, date)
	if e != nil {
		return nil, e
	}
	if a == nil {
		return nil, fail(404, "Manager not found or unassigned")
	}
	rows, e := s.rows(`SELECT a.person_id,p.display_name,pos.code AS position_code
FROM position_reporting r JOIN assignment a ON a.position_id=r.position_id JOIN person p ON p.id=a.person_id
JOIN position pos ON pos.id=a.position_id
WHERE r.tenant_id=? AND r.supervisor_position_id=? AND r.relation_type='LINE' AND `+active("r")+`
AND a.tenant_id=? AND a.assignment_type='primary' AND `+active("a")+` ORDER BY a.person_id`, tenantID, a["position_id"], date, date, tenantID, date, date)
	if e != nil {
		return nil, e
	}
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return map[string]any{"manager_id": person, "as_of": date, "total": total, "limit": limit, "offset": offset, "reports": rows[offset:end]}, nil
}
func (s *Service) resolve(person, date, policy string, levels int) (map[string]any, error) {
	if policy != "LINE_MANAGER" {
		return nil, fail(422, "Only LINE_MANAGER policy is implemented in demo")
	}
	ch, e := s.chain(person, date, 20)
	if e != nil {
		return nil, e
	}
	nodes := ch["chain"].([]map[string]any)
	if len(nodes) >= levels {
		return map[string]any{"as_of": date, "policy": policy, "requested_level": levels, "subject_id": person, "status": "resolved", "assignee_id": nodes[levels-1]["person_id"], "position_code": nodes[levels-1]["position_code"], "full_chain": nodes}, nil
	}
	return map[string]any{"as_of": date, "policy": policy, "requested_level": levels, "subject_id": person, "status": "unresolved", "reason": ch["status"], "assignee_id": nil, "full_chain": nodes}, nil
}
func (s *Service) tenant() (map[string]any, error) {
	v, e := s.one(`SELECT id,code,name FROM tenant WHERE id=?`, tenantID)
	if e != nil {
		return nil, e
	}
	if v == nil {
		return nil, fail(503, "Demo tenant not initialized")
	}
	return map[string]any{"id": v["id"], "code": v["code"], "name": v["name"], "isolation": "database-per-tenant", "mode": "synthetic-demo"}, nil
}
func (s *Service) companies() (map[string]any, error) {
	rs, e := s.rows(`SELECT id,code,name,jurisdiction FROM legal_entity WHERE tenant_id=? ORDER BY code`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"tenant_id": tenantID, "items": rs}, nil
}

var companyCode = regexp.MustCompile(`^[A-Za-z0-9-]{2,32}$`)

func (s *Service) audit(event, target string, payload any) error {
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	return s.db.Exec(`INSERT INTO audit_log(tenant_id,happened_at,actor,event_name,target,payload) VALUES (?,?,?,?,?,?)`, tenantID, time.Now().UTC().Format(time.RFC3339), "demo_api", event, target, string(b))
}
func (s *Service) transaction(fn func() (any, error)) (out any, err error) {
	if e := s.db.Exec("BEGIN IMMEDIATE"); e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			_ = s.db.Exec("ROLLBACK")
		} else {
			if e := s.db.Exec("COMMIT"); e != nil {
				err = e
				_ = s.db.Exec("ROLLBACK")
			}
		}
	}()
	out, err = fn()
	return
}
func (s *Service) addCompany(code, name, place string) (any, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	place = strings.ToUpper(strings.TrimSpace(place))
	if !companyCode.MatchString(code) || len(name) < 2 || len(name) > 200 || len(place) < 2 || len(place) > 8 {
		return nil, fail(422, "Invalid company code/name/jurisdiction")
	}
	return s.transaction(func() (any, error) {
		old, e := s.one(`SELECT id FROM legal_entity WHERE tenant_id=? AND code=?`, tenantID, code)
		if e != nil {
			return nil, e
		}
		if old != nil {
			return nil, fail(409, "Company code already exists")
		}
		id := uid("LE")
		if e = s.db.Exec(`INSERT INTO legal_entity(id,tenant_id,code,name,jurisdiction) VALUES (?,?,?,?,?)`, id, tenantID, code, name, place); e != nil {
			return nil, e
		}
		if e = s.audit("company.create", id, map[string]any{"code": code, "name": name}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "tenant_id": tenantID, "code": code, "name": name, "jurisdiction": place, "synthetic_demo": true}, nil
	})
}
func (s *Service) changes() ([]Row, error) {
	return s.rows(`SELECT id,title,effective_date,status,scenario FROM change_set WHERE tenant_id=? AND domain='org_structure' ORDER BY effective_date,id`, tenantID)
}
func (s *Service) position(code, date string) (Row, error) {
	return s.one(`SELECT p.id,p.code,p.holder_limit,u.legal_entity_id FROM position p JOIN position_placement pl ON pl.position_id=p.id AND pl.tenant_id=p.tenant_id JOIN org_unit u ON u.id=pl.org_unit_id WHERE p.tenant_id=? AND p.code=? AND p.created_at<=? AND `+active("pl"), tenantID, code, date, date, date)
}
func (s *Service) move(person, code, date, reason string) (any, error) {
	if reason == "" {
		reason = "transfer"
	}
	if len(reason) > 64 {
		return nil, fail(422, "reason too long")
	}
	return s.transaction(func() (any, error) {
		if e := s.requirePerson(person); e != nil {
			return nil, e
		}
		p, e := s.position(code, date)
		if e != nil {
			return nil, e
		}
		if p == nil {
			return nil, fail(404, "Target position not active")
		}
		old, e := s.assignment(person, date)
		if e != nil {
			return nil, e
		}
		if old == nil {
			return nil, fail(409, "No active primary assignment")
		}
		if old["position_id"] == p["id"] {
			return nil, fail(409, "Already assigned")
		}
		if date <= rowString(old, "valid_from") || old["valid_to"] != nil {
			return nil, fail(409, "Historical or overlapping move not supported")
		}
		future, e := s.one(`SELECT id FROM assignment WHERE tenant_id=? AND person_id=? AND assignment_type='primary' AND valid_from>=?`, tenantID, person, date)
		if e != nil {
			return nil, e
		}
		if future != nil {
			return nil, fail(409, "Future assignment exists")
		}
		legal, e := s.one(`SELECT legal_entity_id FROM employment_episode WHERE id=? AND tenant_id=?`, old["employment_id"], tenantID)
		if e != nil {
			return nil, e
		}
		if legal["legal_entity_id"] != p["legal_entity_id"] {
			return nil, fail(409, "Cross-company moves require mobility case")
		}
		occupied, e := s.one(`SELECT COUNT(*) AS n FROM assignment WHERE tenant_id=? AND position_id=? AND `+active(""), tenantID, p["id"], date, date)
		if e != nil {
			return nil, e
		}
		if intval(occupied["n"]) >= intval(p["holder_limit"]) {
			return nil, fail(409, "Target position at capacity")
		}
		change := uid("CS")
		now := time.Now().UTC().Format(time.RFC3339)
		if e = s.db.Exec(`INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at) VALUES (?,?,?,?,?,'published',?)`, change, tenantID, "workforce", "Move "+person, date, now); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`UPDATE assignment SET valid_to=? WHERE id=?`, date, old["id"]); e != nil {
			return nil, e
		}
		if e = s.db.Exec(`INSERT INTO assignment(id,tenant_id,person_id,employment_id,position_id,assignment_type,fte,valid_from,change_set_id) VALUES (?,?,?,?,?,'primary',1,?,?)`, uid("ASS"), tenantID, person, old["employment_id"], p["id"], date, change); e != nil {
			return nil, e
		}
		details, _ := json.Marshal(map[string]string{"from": rowString(old, "position_code"), "to": code})
		if e = s.db.Exec(`INSERT INTO hr_event(id,tenant_id,person_id,event_type,effective_date,status,change_set_id,details) VALUES (?,?,?,?,?,'published',?,?)`, uid("EV"), tenantID, person, reason, date, change, string(details)); e != nil {
			return nil, e
		}
		if e = s.audit("assignment.move", person, map[string]string{"from": rowString(old, "position_code"), "to": code, "effective_date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"change_set_id": change, "person_id": person, "effective_date": date, "from_position": old["position_code"], "to_position": code}, nil
	})
}
func (s *Service) cycle(pos, parent, date string) (bool, error) {
	seen := map[string]bool{}
	curr := parent
	for curr != "" {
		if curr == pos || seen[curr] {
			return true, nil
		}
		seen[curr] = true
		rel, e := s.rows(`SELECT supervisor_position_id FROM position_reporting WHERE tenant_id=? AND position_id=? AND relation_type='LINE' AND `+active(""), tenantID, curr, date, date)
		if e != nil {
			return false, e
		}
		if len(rel) > 1 {
			return false, fail(409, "Ambiguous reporting")
		}
		if len(rel) == 0 {
			break
		}
		curr = rowString(rel[0], "supervisor_position_id")
	}
	return false, nil
}
func (s *Service) changeReporting(code, parentCode, date string) (any, error) {
	return s.transaction(func() (any, error) {
		pos, e := s.position(code, date)
		if e != nil {
			return nil, e
		}
		parent, e := s.position(parentCode, date)
		if e != nil {
			return nil, e
		}
		if pos == nil || parent == nil {
			return nil, fail(404, "Unknown/inactive position")
		}
		pid := rowString(pos, "id")
		sid := rowString(parent, "id")
		if pid == sid {
			return nil, fail(409, "Self-reporting not allowed")
		}
		rel, e := s.rows(`SELECT id,valid_from,valid_to,supervisor_position_id FROM position_reporting WHERE tenant_id=? AND position_id=? AND relation_type='LINE' AND `+active(""), tenantID, pid, date, date)
		if e != nil {
			return nil, e
		}
		if len(rel) > 1 {
			return nil, fail(409, "Ambiguous existing reporting")
		}
		if len(rel) > 0 {
			if rowString(rel[0], "supervisor_position_id") == sid {
				return nil, fail(409, "Already reports to supervisor")
			}
			if date <= rowString(rel[0], "valid_from") || rel[0]["valid_to"] != nil {
				return nil, fail(409, "Historical edit not supported")
			}
		}
		future, e := s.one(`SELECT id FROM position_reporting WHERE tenant_id=? AND position_id=? AND relation_type='LINE' AND valid_from>=?`, tenantID, pid, date)
		if e != nil {
			return nil, e
		}
		if future != nil {
			return nil, fail(409, "Future change exists")
		}
		cyclic, e := s.cycle(pid, sid, date)
		if e != nil {
			return nil, e
		}
		if cyclic {
			return nil, fail(409, "Reporting cycle")
		}
		change := uid("CS")
		now := time.Now().UTC().Format(time.RFC3339)
		if e = s.db.Exec(`INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at) VALUES (?,?,?,?,?,'published',?)`, change, tenantID, "org_structure", "Change reporting line", date, now); e != nil {
			return nil, e
		}
		if len(rel) > 0 {
			if e = s.db.Exec(`UPDATE position_reporting SET valid_to=? WHERE id=?`, date, rel[0]["id"]); e != nil {
				return nil, e
			}
		}
		if e = s.db.Exec(`INSERT INTO position_reporting(id,tenant_id,position_id,supervisor_position_id,relation_type,valid_from,change_set_id) VALUES (?,?,?,?,'LINE',?,?)`, uid("REL"), tenantID, pid, sid, date, change); e != nil {
			return nil, e
		}
		if e = s.audit("org.reporting_change", code, map[string]string{"to": parentCode, "effective_date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"change_set_id": change, "position": code, "supervisor_position": parentCode, "effective_date": date}, nil
	})
}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	page := func(asset string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				http.Error(w, "Method not allowed", 405)
				return
			}
			b, e := builtIn.ReadFile("assets/" + asset)
			if e != nil {
				http.Error(w, "Asset missing", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Write(b)
		}
	}
	mux.HandleFunc("GET /assets/app.css", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/app.css")
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("GET /assets/revamp.css", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/revamp.css")
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("GET /assets/revamp.js", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/revamp.js")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("GET /assets/app.js", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/app.js")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("GET /assets/avatars/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !regexp.MustCompile(`^employee-\d{2}\.webp$`).MatchString(name) {
			http.NotFound(w, r)
			return
		}
		b, e := builtIn.ReadFile("assets/avatars/" + name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(b)
	})
	mux.HandleFunc("GET /assets/illustrations/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !regexp.MustCompile(`^hero_[a-z_]+\.webp$`).MatchString(name) {
			http.NotFound(w, r)
			return
		}
		b, e := builtIn.ReadFile("assets/illustrations/" + name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(b)
	})
	mux.HandleFunc("GET /{$}", page("hero.html"))
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		if s.secureDemo {
			http.Redirect(w, r, "/workspace", http.StatusSeeOther)
			return
		}
		page("index.html")(w, r)
	})
	mux.HandleFunc("GET /legacy", func(w http.ResponseWriter, r *http.Request) {
		if s.secureDemo {
			s.mu.Lock()
			p, e := s.principal(r)
			s.mu.Unlock()
			if e != nil {
				handleError(w, e)
				return
			}
			if !role(p, "ADMIN") {
				http.Error(w, "Admin legacy view only", 403)
				return
			}
		}
		page("index.html")(w, r)
	})
	mux.HandleFunc("GET /workspace", page("workspace.html"))
	mux.HandleFunc("GET /assets/workspace.css", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/workspace.css")
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /assets/workspace.js", func(w http.ResponseWriter, r *http.Request) {
		b, _ := builtIn.ReadFile("assets/workspace.js")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(b)
	})
	s.authRoutes(mux)
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		s.protected(w, r, func(p *Principal) (any, error) { return s.v6API(p, r) })
	})
	mux.HandleFunc("GET /docs", page("docs.html"))
	mux.HandleFunc("GET /logs", page("logs.html"))
	mux.HandleFunc("GET /v1/system/status", func(w http.ResponseWriter, r *http.Request) {
		s.api(w, r, func() (any, error) {
			return map[string]any{"status": "running", "port": s.port, "uptime_seconds": int(time.Since(s.started).Seconds()), "logs": "data/orgaxis.log", "mode": "synthetic-demo"}, nil
		})
	})
	mux.HandleFunc("GET /v1/system/logs", func(w http.ResponseWriter, r *http.Request) {
		s.api(w, r, func() (any, error) {
			n, e := takeInt(r, "lines", 100, 1, 500)
			if e != nil {
				return nil, e
			}
			if s.logfile == nil {
				return map[string]any{"lines": []string{}, "count": 0}, nil
			}
			lines, e := s.logfile.Tail(n)
			if e != nil {
				return nil, e
			}
			return map[string]any{"lines": lines, "count": len(lines)}, nil
		})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response(w, 200, map[string]any{"status": "ok", "mode": "local-synthetic-demo", "synthetic_data": true, "database": "sqlite", "port": s.port, "uptime_seconds": int(time.Since(s.started).Seconds())})
	})
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		s.api(w, r, func() (any, error) {
			p := r.URL.Path
			switch {
			case p == "/v1/tenant" && r.Method == "GET":
				return s.tenant()
			case p == "/v1/companies" && r.Method == "GET":
				return s.companies()
			case p == "/v1/companies" && r.Method == "POST":
				var v struct {
					Code         string `json:"code"`
					Name         string `json:"name"`
					Jurisdiction string `json:"jurisdiction"`
				}
				if e := bodyJSON(r, &v); e != nil {
					return nil, e
				}
				if v.Jurisdiction == "" {
					v.Jurisdiction = "VN"
				}
				return s.addCompany(v.Code, v.Name, v.Jurisdiction)
			case p == "/v1/org/blank" && r.Method == "GET":
				d, e := argDate(r)
				if e != nil {
					return nil, e
				}
				return s.blankOrg(d)
			case p == "/v1/org/occupied" && r.Method == "GET":
				d, e := argDate(r)
				if e != nil {
					return nil, e
				}
				limit, e := takeInt(r, "limit", 100, 1, 1000)
				if e != nil {
					return nil, e
				}
				offset, e := takeInt(r, "offset", 0, 0, 1000000)
				if e != nil {
					return nil, e
				}
				return s.occupied(d, limit, offset)
			case p == "/v1/org/changes" && r.Method == "GET":
				return s.changes()
			case p == "/v1/workflow/resolve" && r.Method == "GET":
				d, e := argDate(r)
				if e != nil {
					return nil, e
				}
				levels, e := takeInt(r, "levels", 1, 1, 10)
				if e != nil {
					return nil, e
				}
				policy := r.URL.Query().Get("policy")
				if policy == "" {
					policy = "LINE_MANAGER"
				}
				return s.resolve(r.URL.Query().Get("subject_id"), d, policy, levels)
			case p == "/v1/assignments/move" && r.Method == "POST":
				var v struct {
					PersonID string `json:"person_id"`
					Code     string `json:"target_position_code"`
					Date     string `json:"effective_date"`
					Reason   string `json:"reason"`
				}
				if e := bodyJSON(r, &v); e != nil {
					return nil, e
				}
				d, e := requireDate(v.Date)
				if e != nil {
					return nil, e
				}
				return s.move(v.PersonID, v.Code, d, v.Reason)
			case p == "/v1/org/reporting/change" && r.Method == "POST":
				var v struct {
					Code       string `json:"position_code"`
					Supervisor string `json:"supervisor_position_code"`
					Date       string `json:"effective_date"`
				}
				if e := bodyJSON(r, &v); e != nil {
					return nil, e
				}
				d, e := requireDate(v.Date)
				if e != nil {
					return nil, e
				}
				return s.changeReporting(v.Code, v.Supervisor, d)
			case (p == "/v1/people/search" || strings.HasPrefix(p, "/v1/people/detail/")) && r.Method == "GET":
				return s.moduleAPI(r)
			case strings.HasPrefix(p, "/v1/people/") && r.Method == "GET":
				bits := strings.Split(strings.Trim(p, "/"), "/")
				if len(bits) != 4 {
					return nil, fail(404, "Unknown endpoint")
				}
				person := bits[2]
				date, e := argDate(r)
				if e != nil {
					return nil, e
				}
				switch bits[3] {
				case "manager":
					return s.manager(person, date)
				case "chain":
					n, e := takeInt(r, "max_levels", 20, 1, 50)
					if e != nil {
						return nil, e
					}
					return s.chain(person, date, n)
				case "reports":
					limit, e := takeInt(r, "limit", 100, 1, 1000)
					if e != nil {
						return nil, e
					}
					offset, e := takeInt(r, "offset", 0, 0, 1000000)
					if e != nil {
						return nil, e
					}
					return s.directReports(person, date, limit, offset)
				}
			}
			return s.moduleAPI(r)
		})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'; base-uri 'none'; object-src 'none'")
		mux.ServeHTTP(w, r)
	})
}
func setupDB(dir string) (string, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	name := filepath.Join(dir, "orgaxis.sqlite3")
	if _, e := os.Stat(name); errors.Is(e, os.ErrNotExist) {
		data, e := builtIn.ReadFile("assets/seed.sqlite3")
		if e != nil {
			return "", e
		}
		temp := name + ".tmp"
		if e := os.WriteFile(temp, data, 0600); e != nil {
			return "", e
		}
		if e = os.Rename(temp, name); e != nil {
			return "", e
		}
		return name, nil
	} else if e != nil {
		return "", e
	}
	return name, nil
}
func launchBrowser(url string) {
	if runtime.GOOS == "windows" {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}
