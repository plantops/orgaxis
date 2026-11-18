//go:build linux && cgo

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	temp := t.TempDir()
	dbPath, e := setupDB(filepath.Join(temp, "data"))
	if e != nil {
		t.Fatal(e)
	}
	db, e := openSQLite(dbPath)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return &Service{db: db, key: "demo-local-key", port: 8765}
}
func serve(t *testing.T, s *Service, method, path, body string, key bool) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if key {
		req.Header.Set("X-API-Key", "demo-local-key")
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	s.routes().ServeHTTP(res, req)
	return res.Code, res.Body.Bytes()
}
func obj(t *testing.T, b []byte) map[string]any {
	t.Helper()
	v := map[string]any{}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestServeHeroAndApp(t *testing.T) {
	s := testService(t)
	for _, p := range []string{"/", "/app", "/docs", "/health"} {
		status, html := serve(t, s, "GET", p, "", false)
		if status != 200 {
			t.Fatalf("%s returned %d", p, status)
		}
		if len(html) < 30 {
			t.Fatalf("%s empty", p)
		}
	}
	_, hero := serve(t, s, "GET", "/", "", false)
	if !bytes.Contains(hero, []byte("Tổ chức luôn thay đổi")) || !bytes.Contains(hero, []byte("data-date")) {
		t.Fatal("missing hero copy or interactive timeline")
	}
}
func TestHealthPort(t *testing.T) {
	s := testService(t)
	status, data := serve(t, s, "GET", "/health", "", false)
	if status != 200 || int(obj(t, data)["port"].(float64)) != 8765 {
		t.Fatalf("health status or port mismatch: %d %s", status, data)
	}
}
func TestAuthTenantAndCompany(t *testing.T) {
	s := testService(t)
	status, _ := serve(t, s, "GET", "/v1/tenant", "", false)
	if status != 401 {
		t.Fatalf("unauth status=%d", status)
	}
	status, b := serve(t, s, "GET", "/v1/tenant", "", true)
	if status != 200 || obj(t, b)["id"] != tenantID {
		t.Fatalf("tenant=%d %s", status, b)
	}
	status, b = serve(t, s, "GET", "/v1/companies", "", true)
	if status != 200 || len(obj(t, b)["items"].([]any)) != 1 {
		t.Fatalf("companies=%d %s", status, b)
	}
	status, b = serve(t, s, "POST", "/v1/companies", `{"code":"NEWCO","name":"New Synthetic Company","jurisdiction":"VN"}`, true)
	if status != 201 {
		t.Fatalf("create company=%d %s", status, b)
	}
	status, _ = serve(t, s, "POST", "/v1/companies", `{"code":"NEWCO","name":"Duplicated","jurisdiction":"VN"}`, true)
	if status != 409 {
		t.Fatalf("duplicate status=%d", status)
	}
}
func TestTimelineAndManager(t *testing.T) {
	s := testService(t)
	for date, expected := range map[string]int{"2026-06-30": 8, "2027-01-01": 9} {
		status, b := serve(t, s, "GET", "/v1/org/blank?as_of="+date, "", true)
		if status != 200 || int(obj(t, b)["unit_count"].(float64)) != expected {
			t.Fatalf("%s %d %s", date, status, b)
		}
	}
	for date, expected := range map[string]string{"2026-02-01": "EMP-0009", "2026-11-18": "EMP-0010"} {
		status, b := serve(t, s, "GET", "/v1/people/EMP-0030/manager?as_of="+date, "", true)
		if status != 200 || obj(t, b)["manager"].(map[string]any)["person_id"] != expected {
			t.Fatalf("%s %d %s", date, status, b)
		}
	}
	status, b := serve(t, s, "GET", "/v1/org/changes", "", true)
	if status != 200 {
		t.Fatal(status)
	}
	var changes []any
	if e := json.Unmarshal(b, &changes); e != nil || len(changes) != 2 {
		t.Fatalf("expected 2 org changes %s", b)
	}
	status, b = serve(t, s, "GET", "/v1/workflow/resolve?subject_id=EMP-0030&levels=1&as_of=2026-11-18", "", true)
	if status != 200 || obj(t, b)["assignee_id"] != "EMP-0010" {
		t.Fatalf("resolve %d %s", status, b)
	}
}
func TestMovesAndReportingCycles(t *testing.T) {
	s := testService(t)
	status, b := serve(t, s, "POST", "/v1/assignments/move", `{"person_id":"EMP-0200","target_position_code":"PACK-OP","effective_date":"2026-12-05"}`, true)
	if status != 201 {
		t.Fatalf("move %d %s", status, b)
	}
	status, b = serve(t, s, "GET", "/v1/people/EMP-0200/manager?as_of=2026-12-06", "", true)
	if status != 200 || obj(t, b)["position_code"] != "PACK-OP" {
		t.Fatalf("moved manager %d %s", status, b)
	}
	status, b = serve(t, s, "POST", "/v1/org/reporting/change", `{"position_code":"WH-MGR","supervisor_position_code":"WH-LEAD","effective_date":"2026-12-06"}`, true)
	if status != 409 {
		t.Fatalf("cycle rejected status=%d %s", status, b)
	}
}
func TestInputValidation(t *testing.T) {
	s := testService(t)
	for _, p := range []string{"/v1/org/blank?as_of=2026-02-99", "/v1/org/occupied?limit=9999", "/v1/people/NOT-FOUND/manager?as_of=2026-11-18"} {
		status, _ := serve(t, s, "GET", p, "", true)
		if status == 200 {
			t.Fatalf("expected error for %s", p)
		}
	}
}
func TestPreloadedDB(t *testing.T) {
	f, e := builtIn.Open("assets/seed.sqlite3")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || stat.Size() < 100000 {
		t.Fatalf("bad embedded db %v", e)
	}
	_, _ = io.Copy(io.Discard, f)
	_, e = os.Stat("assets/hero.html")
	if e != nil {
		t.Fatal(e)
	}
}
