package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// v0.6 is a local, synthetic-data RBAC pilot, not SSO or certified HR security.
// All decision checks are server-side; roles, scope and person ID are derived
// from a database-backed session and never accepted from a request payload.
type Principal struct{ UserID, PersonID, Role, CSRF string }

var loginGuard = struct {
	sync.Mutex
	failures map[string][]time.Time
}{failures: map[string][]time.Time{}}

func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func sha(s string) string { x := sha256.Sum256([]byte(s)); return hex.EncodeToString(x[:]) }

// PBKDF2-HMAC-SHA256 in stdlib to keep Windows binary dependency free.
func derive(password, salt string, rounds int) string {
	key := []byte(password)
	msg := append([]byte(salt), 0, 0, 0, 1)
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	u := h.Sum(nil)
	t := append([]byte(nil), u...)
	for i := 1; i < rounds; i++ {
		h.Reset()
		h.Write(u)
		u = h.Sum(nil)
		for j := range t {
			t[j] ^= u[j]
		}
	}
	return hex.EncodeToString(t)
}
func secureSchema(db SQLiteDB) error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS oa_user(id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,username TEXT NOT NULL UNIQUE,person_id TEXT NOT NULL,role TEXT NOT NULL CHECK(role IN ('EMPLOYEE','SUPERVISOR','HR','PAYROLL','DIRECTOR','ADMIN')),salt TEXT NOT NULL,passhash TEXT NOT NULL,active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oa_session(token_hash TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES oa_user(id),csrf TEXT NOT NULL,expires_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS ix_oa_session_exp ON oa_session(expires_at)`,
		`CREATE TABLE IF NOT EXISTS oa_case(id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,subject_id TEXT NOT NULL,opened_by TEXT NOT NULL,case_type TEXT NOT NULL CHECK(case_type IN ('LEAVE','BONUS','DISCIPLINE')),title TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',category TEXT NOT NULL DEFAULT '',start_date TEXT,end_date TEXT,workdays INTEGER NOT NULL DEFAULT 0,net_benefit INTEGER NOT NULL DEFAULT 0,impact_level INTEGER NOT NULL DEFAULT 0,annex03 INTEGER NOT NULL DEFAULT 0,reward_estimate INTEGER NOT NULL DEFAULT 0,reward_tier TEXT NOT NULL DEFAULT '',stage TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'OPEN',created_at TEXT NOT NULL,closed_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS oa_case_step(id TEXT PRIMARY KEY,case_id TEXT NOT NULL REFERENCES oa_case(id),seq INTEGER NOT NULL,role TEXT NOT NULL,assignee_id TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'WAITING',decided_by TEXT,decision_at TEXT,note TEXT NOT NULL DEFAULT '',UNIQUE(case_id,seq))`,
		`CREATE INDEX IF NOT EXISTS ix_oa_case_subject ON oa_case(tenant_id,subject_id,created_at)`,
		`CREATE INDEX IF NOT EXISTS ix_oa_case_step ON oa_case_step(case_id,status)`,
		`CREATE TABLE IF NOT EXISTS oa_audit(id TEXT PRIMARY KEY,actor_id TEXT NOT NULL,event TEXT NOT NULL,object_id TEXT NOT NULL,occurred_at TEXT NOT NULL,details TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oa_policy(tenant_id TEXT NOT NULL,key TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(tenant_id,key))`,
	}
	for _, q := range ddl {
		if err := db.Exec(q); err != nil {
			return err
		}
	}
	// Additive migration if a pilot case table was already created by an earlier preview.
	cols, err := db.Query(`PRAGMA table_info(oa_case)`)
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for _, c := range cols {
		found[rowString(c, "name")] = true
	}
	if !found["impact_level"] {
		if err := db.Exec(`ALTER TABLE oa_case ADD COLUMN impact_level INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !found["annex03"] {
		if err := db.Exec(`ALTER TABLE oa_case ADD COLUMN annex03 INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	for k, v := range map[string]string{"leave_direct_days": "2", "leave_hr_days": "5", "work_calendar": "MON_SAT", "reward_rule": "KAIZEN_DEMO_2026"} {
		if err := db.Exec(`INSERT OR IGNORE INTO oa_policy(tenant_id,key,value) VALUES(?,?,?)`, tenantID, k, v); err != nil {
			return err
		}
	}
	return nil
}
func bootstrapUsers(db SQLiteDB, dir string) error {
	// Never reset credentials on an existing installation.
	existing, err := first(db, `SELECT COUNT(*) AS n FROM oa_user WHERE tenant_id=?`, tenantID)
	if err != nil {
		return err
	}
	if intval(existing["n"]) != 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	accounts := []struct{ user, person, role string }{
		{"admin", "EMP-0002", "ADMIN"}, {"director", "EMP-0001", "DIRECTOR"},
		{"hr", "EMP-0008", "HR"}, {"payroll", "EMP-0007", "PAYROLL"},
		{"manager", "EMP-0009", "SUPERVISOR"}, {"employee", "EMP-0012", "EMPLOYEE"},
	}
	var lines []string
	if err := db.Exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = db.Exec("ROLLBACK")
		}
	}()
	for _, a := range accounts {
		password := token()[:24]
		salt := token()[:32]
		hash := derive(password, salt, 100000)
		err = db.Exec(`INSERT INTO oa_user(id,tenant_id,username,person_id,role,salt,passhash,created_at) VALUES(?,?,?,?,?,?,?,?)`, uid("USR"), tenantID, a.user, a.person, a.role, salt, hash, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%-12s %-12s %-12s %s", a.user, a.person, a.role, password))
	}
	// Credentials file is local demo bootstrap only; never commit or serve it.
	credfile := filepath.Join(dir, "DEMO-LOGIN-PRIVATE.txt")
	msg := "ORGAXIS LOCAL SYNTHETIC DEMO — RANDOM INITIAL CREDENTIALS\nNot for real personnel data. Keep this file private.\n\nUSERNAME     PERSON ID    ROLE         PASSWORD\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(credfile, []byte(msg), 0600); err != nil {
		return err
	}
	if err := db.Exec("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
func (s *Service) principal(r *http.Request) (*Principal, error) {
	cookie, err := r.Cookie("oa_session")
	if err != nil || len(cookie.Value) != 64 {
		return nil, fail(401, "Sign in required")
	}
	row, err := s.one(`SELECT u.id,u.person_id,u.role,s.csrf FROM oa_session s JOIN oa_user u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>? AND u.active=1 AND u.tenant_id=?`, sha(cookie.Value), time.Now().UTC().Format(time.RFC3339), tenantID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fail(401, "Session expired")
	}
	return &Principal{rowString(row, "id"), rowString(row, "person_id"), rowString(row, "role"), rowString(row, "csrf")}, nil
}
func role(p *Principal, roles ...string) bool {
	if p == nil {
		return false
	}
	for _, v := range roles {
		if v == p.Role {
			return true
		}
	}
	return false
}
func (s *Service) subordinate(actor, target, date string) bool {
	if actor == target {
		return false
	}
	chain, err := s.chain(target, date, 30)
	if err != nil {
		return false
	}
	v, ok := chain["chain"].([]map[string]any)
	if !ok {
		return false
	}
	for _, x := range v {
		if x["person_id"] == actor {
			return true
		}
	}
	return false
}
func (s *Service) canSeePerson(p *Principal, target, date string) bool {
	return p.PersonID == target || role(p, "ADMIN", "HR") || (role(p, "SUPERVISOR", "DIRECTOR") && s.subordinate(p.PersonID, target, date))
}
func (s *Service) canSeePayroll(p *Principal, target string) bool {
	return p.PersonID == target || role(p, "PAYROLL", "ADMIN")
}
func auditV6(s *Service, actor, event, object string, details any) error {
	v, _ := json.Marshal(details)
	return s.db.Exec(`INSERT INTO oa_audit VALUES(?,?,?,?,?,?)`, uid("AUD"), actor, event, object, time.Now().UTC().Format(time.RFC3339), string(v))
}
func loginAllowed(k string) bool {
	loginGuard.Lock()
	defer loginGuard.Unlock()
	now := time.Now()
	var current []time.Time
	for _, t := range loginGuard.failures[k] {
		if now.Sub(t) < 10*time.Minute {
			current = append(current, t)
		}
	}
	loginGuard.failures[k] = current
	return len(current) < 5
}
func loginFailure(k string) {
	loginGuard.Lock()
	defer loginGuard.Unlock()
	loginGuard.failures[k] = append(loginGuard.failures[k], time.Now())
}
func loginClear(k string) {
	loginGuard.Lock()
	defer loginGuard.Unlock()
	delete(loginGuard.failures, k)
}
func (s *Service) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := bodyJSON(r, &v); err != nil {
			handleError(w, err)
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := strings.ToLower(strings.TrimSpace(v.Username)) + ":" + host
		if !loginAllowed(key) {
			response(w, 429, map[string]any{"detail": "Too many attempts"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		row, err := s.one(`SELECT id,person_id,role,salt,passhash FROM oa_user WHERE username=? AND tenant_id=? AND active=1`, strings.ToLower(strings.TrimSpace(v.Username)), tenantID)
		dummy := derive(v.Password, "00000000000000000000000000000000", 100000)
		valid := false
		if err == nil && row != nil {
			expected := rowString(row, "passhash")
			actual := derive(v.Password, rowString(row, "salt"), 100000)
			valid = subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
		} else {
			_ = dummy
		}
		if !valid {
			loginFailure(key)
			response(w, 401, map[string]any{"detail": "Invalid credentials"})
			return
		}
		loginClear(key)
		secret := token()
		csrf := token()
		expires := time.Now().UTC().Add(8 * time.Hour).Format(time.RFC3339)
		if err := s.db.Exec(`INSERT INTO oa_session(token_hash,user_id,csrf,expires_at) VALUES(?,?,?,?)`, sha(secret), row["id"], csrf, expires); err != nil {
			handleError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "oa_session", Value: secret, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: time.Now().Add(8 * time.Hour)})
		_ = auditV6(s, rowString(row, "person_id"), "LOGIN", rowString(row, "id"), map[string]any{"role": row["role"]})
		response(w, 200, map[string]any{"person_id": row["person_id"], "role": row["role"], "csrf": csrf})
	})
	mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		p, e := s.principal(r)
		if e != nil {
			handleError(w, e)
			return
		}
		name, _ := s.one(`SELECT display_name FROM person WHERE id=?`, p.PersonID)
		response(w, 200, map[string]any{"person_id": p.PersonID, "name": name["display_name"], "role": p.Role, "csrf": p.CSRF})
	})
	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		p, e := s.principal(r)
		if e != nil {
			handleError(w, e)
			return
		}
		if r.Header.Get("X-CSRF-Token") != p.CSRF {
			response(w, 403, map[string]any{"detail": "CSRF"})
			return
		}
		c, _ := r.Cookie("oa_session")
		_ = s.db.Exec(`DELETE FROM oa_session WHERE token_hash=?`, sha(c.Value))
		http.SetCookie(w, &http.Cookie{Name: "oa_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		response(w, 200, map[string]any{"signed_out": true})
	})
}
func (s *Service) protected(w http.ResponseWriter, r *http.Request, fn func(*Principal) (any, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.principal(r)
	if e != nil {
		handleError(w, e)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if len(p.CSRF) == 0 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(p.CSRF)) != 1 {
			response(w, 403, map[string]any{"detail": "Invalid CSRF"})
			return
		}
	}
	data, err := fn(p)
	if err != nil {
		handleError(w, err)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	response(w, status, data)
}

// Protect existing demo databases against failed v0.6 schema migrations.
func backupBeforeSecurity(db SQLiteDB, dbPath string) (string, error) {
	existing, e := first(db, `SELECT name FROM sqlite_master WHERE type='table' AND name='oa_user'`)
	if e != nil || existing != nil {
		return "", e
	}
	target := filepath.Join(filepath.Dir(dbPath), "orgaxis-before-v06-"+time.Now().UTC().Format("20060102-150405")+".sqlite3")
	if e := db.Exec(`VACUUM INTO ?`, target); e == nil {
		return target, nil
	}
	if e := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); e != nil {
		return "", fmt.Errorf("cannot checkpoint before backup: %w", e)
	}
	source, e := os.Open(dbPath)
	if e != nil {
		return "", e
	}
	defer source.Close()
	destination, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return "", e
	}
	_, e = io.Copy(destination, source)
	if se := destination.Sync(); e == nil {
		e = se
	}
	if ce := destination.Close(); e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(target)
		return "", e
	}
	return target, nil
}
