//go:build linux && cgo

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type secureTest struct {
	service   *Service
	passwords map[string]string
}

func secured(t *testing.T) *secureTest {
	t.Helper()
	dir := t.TempDir()
	path, e := setupDB(filepath.Join(dir, "data"))
	if e != nil {
		t.Fatal(e)
	}
	db, e := openSQLite(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e := enhanceDB(db); e != nil {
		t.Fatal(e)
	}
	if e := upgradePayrollWorkbench(db); e != nil {
		t.Fatal(e)
	}
	if e := secureSchema(db); e != nil {
		t.Fatal(e)
	}
	if e := bootstrapUsers(db, filepath.Dir(path)); e != nil {
		t.Fatal(e)
	}
	if e := seedV6Cases(db); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(path), "DEMO-LOGIN-PRIVATE.txt"))
	if e != nil {
		t.Fatal(e)
	}
	pass := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 4 && len(f[3]) > 15 && (f[2] == "ADMIN" || f[2] == "EMPLOYEE" || f[2] == "SUPERVISOR" || f[2] == "HR" || f[2] == "DIRECTOR" || f[2] == "PAYROLL") {
			pass[f[0]] = f[3]
		}
	}
	if len(pass) != 6 {
		t.Fatalf("expected 6 demo roles, got %d", len(pass))
	}
	return &secureTest{&Service{db: db, key: "demo-local-key", port: 8765, secureDemo: true}, pass}
}

type logged struct {
	cookie *http.Cookie
	csrf   string
}

func call(s *Service, method, path, body string, session *logged, csrf bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != nil {
		req.AddCookie(session.cookie)
		if csrf {
			req.Header.Set("X-CSRF-Token", session.csrf)
		}
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, req)
	return w
}
func signin(t *testing.T, v *secureTest, name string) *logged {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": name, "password": v.passwords[name]})
	w := call(v.service, "POST", "/auth/login", string(body), nil, false)
	if w.Code != 200 {
		t.Fatalf("login %s: %d: %s", name, w.Code, w.Body.String())
	}
	payload := obj(t, w.Body.Bytes())
	return &logged{w.Result().Cookies()[0], payload["csrf"].(string)}
}
func TestSecureRBACPayrollAndOrg(t *testing.T) {
	v := secured(t)
	employee := signin(t, v, "employee")
	manager := signin(t, v, "manager")
	payroll := signin(t, v, "payroll")
	if x := call(v.service, "GET", "/v2/org/full?as_of=2026-11-18", "", employee, false); x.Code != 200 || int(obj(t, x.Body.Bytes())["total"].(float64)) != 1000 {
		t.Fatalf("not all employees shown: %s", x.Body)
	}
	for _, user := range []*logged{employee, manager} {
		if x := call(v.service, "GET", "/v2/payroll/year", "", user, false); x.Code != 403 {
			t.Fatalf("forbidden payroll must be 403: %d", x.Code)
		}
	}
	if x := call(v.service, "GET", "/v2/payroll/year", "", payroll, false); x.Code != 200 || int(obj(t, x.Body.Bytes())["count"].(float64)) != 12 {
		t.Fatalf("annual payroll incomplete: %s", x.Body)
	}
	if x := call(v.service, "GET", "/v2/payroll/person/EMP-0001", "", employee, false); x.Code != 403 {
		t.Fatalf("salary scope leak: %d", x.Code)
	}
	if x := call(v.service, "GET", "/v2/payroll/person/EMP-0012", "", employee, false); x.Code != 200 || int(obj(t, x.Body.Bytes())["count"].(float64)) != 12 {
		t.Fatalf("own annual salary: %s", x.Body)
	}
	if x := call(v.service, "GET", "/v1/people/search?limit=5", "", employee, false); x.Code != 403 {
		t.Fatalf("legacy v1 was not gated: %d", x.Code)
	}
	if x := call(v.service, "GET", "/v2/people/EMP-0002", "", employee, false); x.Code != 403 {
		t.Fatalf("person scope leak: %d", x.Code)
	}
}
func TestLeaveAuthorizationAndDecisionStages(t *testing.T) {
	v := secured(t)
	employee := signin(t, v, "employee")
	manager := signin(t, v, "manager")
	hr := signin(t, v, "hr")
	director := signin(t, v, "director")
	input := `{"type":"LEAVE","start":"2026-12-04","end":"2026-12-07","title":"Three workday leave"}`
	if x := call(v.service, "POST", "/v2/cases", input, employee, false); x.Code != 403 {
		t.Fatalf("CSRF bypass: %d", x.Code)
	}
	w := call(v.service, "POST", "/v2/cases", input, employee, true)
	if w.Code != 201 {
		t.Fatalf("create leave: %d %s", w.Code, w.Body)
	}
	data := obj(t, w.Body.Bytes())
	id := data["id"].(string)
	if int(data["workdays"].(float64)) != 3 {
		t.Fatalf("calendar wrong: %v", data)
	}
	if len(data["route"].([]any)) != 2 {
		t.Fatalf("expected LINE_MANAGER and HR route: %v", data)
	}
	decision := `{"decision":"APPROVE","note":"Reviewed"}`
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, employee, true); x.Code != 403 {
		t.Fatalf("self approve allowed: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, hr, true); x.Code != 403 {
		t.Fatalf("out of order HR approve: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, manager, true); x.Code != 201 {
		t.Fatalf("manager approval: %d %s", x.Code, x.Body)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, director, true); x.Code != 403 {
		t.Fatalf("director prematurely approved: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, hr, true); x.Code != 201 || obj(t, x.Body.Bytes())["status"] != "APPROVED" {
		t.Fatalf("HR final approval: %d %s", x.Code, x.Body)
	}
}
func TestKaizenManagerStartsBonusAndHRReviews(t *testing.T) {
	v := secured(t)
	employee := signin(t, v, "employee")
	manager := signin(t, v, "manager")
	hr := signin(t, v, "hr")
	director := signin(t, v, "director")
	payload := `{"type":"BONUS","subject_id":"EMP-0012","title":"Reduction in power consumption","description":"Measured net annual benefit demo","category":"FINANCIAL","net_benefit":50000000}`
	if x := call(v.service, "POST", "/v2/cases", payload, employee, true); x.Code != 403 {
		t.Fatalf("employee initiated bonus: %d", x.Code)
	}
	w := call(v.service, "POST", "/v2/cases", payload, manager, true)
	if w.Code != 201 {
		t.Fatalf("manager created bonus: %d %s", w.Code, w.Body)
	}
	data := obj(t, w.Body.Bytes())
	id := data["id"].(string)
	if data["reward_tier"] != "T4" || int(data["estimate"].(float64)) != 5_000_000 {
		t.Fatalf("Kaizen benchmark tier incorrect: %v", data)
	}
	decision := `{"decision":"APPROVE","note":"Evaluation complete"}`
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, manager, true); x.Code != 403 {
		t.Fatalf("initiator wrongly approved review: %d", x.Code)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, hr, true); x.Code != 201 {
		t.Fatalf("HR review failed: %d %s", x.Code, x.Body)
	}
	if x := call(v.service, "POST", "/v2/cases/"+id+"/decision", decision, director, true); x.Code != 201 || obj(t, x.Body.Bytes())["status"] != "APPROVED" {
		t.Fatalf("director review failed: %d %s", x.Code, x.Body)
	}
}

func TestTemporalPersonReportingAndPolicy(t *testing.T) {
	v := secured(t)
	employee := signin(t, v, "employee")
	admin := signin(t, v, "admin")
	for date, manager := range map[string]string{"2026-02-01": "EMP-0009", "2026-11-18": "EMP-0010"} {
		w := call(v.service, "GET", "/v2/org/full?as_of="+date, "", employee, false)
		if w.Code != 200 {
			t.Fatalf("org graph %s: %s", date, w.Body)
		}
		people := obj(t, w.Body.Bytes())["people"].([]any)
		for _, value := range people {
			p := value.(map[string]any)
			if p["person_id"] == "EMP-0030" && p["manager_person_id"] != manager {
				t.Fatalf("temporal manager %s = %v, want %s", date, p["manager_person_id"], manager)
			}
		}
	}
	w := call(v.service, "POST", "/v2/policies", `{"direct_days":3,"hr_days":6}`, employee, true)
	if w.Code != 403 {
		t.Fatalf("worker policy update allowed: %d", w.Code)
	}
	w = call(v.service, "POST", "/v2/policies", `{"direct_days":3,"hr_days":6}`, admin, true)
	if w.Code != 201 {
		t.Fatalf("admin policy update: %d %s", w.Code, w.Body)
	}
	w = call(v.service, "POST", "/v2/cases", `{"type":"LEAVE","start":"2026-12-04","end":"2026-12-07","title":"three work days"}`, employee, true)
	if w.Code != 201 {
		t.Fatalf("leave: %d %s", w.Code, w.Body)
	}
	if len(obj(t, w.Body.Bytes())["route"].([]any)) != 1 {
		t.Fatal("updated threshold not applied")
	}
}
func TestAccountAdminGrantAndAudit(t *testing.T) {
	v := secured(t)
	admin := signin(t, v, "admin")
	employee := signin(t, v, "employee")
	w := call(v.service, "GET", "/v2/users", "", employee, false)
	if w.Code != 403 {
		t.Fatalf("users disclosed: %d", w.Code)
	}
	payload := `{"username":"testuser","person_id":"EMP-0013","role":"EMPLOYEE"}`
	w = call(v.service, "POST", "/v2/users", payload, employee, true)
	if w.Code != 403 {
		t.Fatalf("RBAC escalation: %d", w.Code)
	}
	w = call(v.service, "POST", "/v2/users", payload, admin, true)
	if w.Code != 201 {
		t.Fatalf("new account: %d %s", w.Code, w.Body)
	}
	if len(obj(t, w.Body.Bytes())["password_once"].(string)) < 20 {
		t.Fatal("weak new password")
	}
	w = call(v.service, "GET", "/v2/audit", "", admin, false)
	if w.Code != 200 {
		t.Fatalf("audit: %d", w.Code)
	}
}
