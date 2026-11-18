//go:build linux && cgo

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOperationalModuleReads(t *testing.T) {
	s := testService(t)
	paths := []string{
		"/v1/dashboard?as_of=2026-11-18", "/v1/people/search?limit=35&as_of=2026-11-18",
		"/v1/people/detail/EMP-0030?as_of=2026-11-18", "/v1/skills/catalog", "/v1/skills/holders?skill=FORK",
		"/v1/training/programs", "/v1/training/records?limit=50", "/v1/performance?period=2026-10",
		"/v1/absences", "/v1/recognitions", "/v1/discipline", "/v1/occupational",
		"/v1/workflow/requests", "/v1/payroll/summary?period=2026-10", "/v1/benefits/claims",
		"/v1/immigration/summary?as_of=2026-11-18", "/v1/modules",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			status, b := serve(t, s, "GET", path, "", true)
			if status != 200 {
				t.Fatalf("status %d: %s", status, string(b))
			}
			if len(b) < 20 {
				t.Fatalf("empty %s", path)
			}
		})
	}
	status, b := serve(t, s, "GET", "/v1/dashboard?as_of=2026-11-18", "", true)
	if status != 200 || int(obj(t, b)["active_people"].(float64)) != 1000 {
		t.Fatalf("expected 1000 active people: %s", b)
	}
	status, b = serve(t, s, "GET", "/v1/skills/catalog", "", true)
	if status != 200 || int(obj(t, b)["people_multiskill"].(float64)) < 800 {
		t.Fatalf("expected cross-skills %s", b)
	}
	status, b = serve(t, s, "GET", "/v1/immigration/summary", "", true)
	if status != 200 || len(obj(t, b)["experts"].([]any)) != 18 {
		t.Fatalf("expected 18 synthetic experts: %s", b)
	}
}
func TestOperationalModuleWrites(t *testing.T) {
	s := testService(t)
	cases := []struct {
		name, path, data string
		want             int
	}{
		{"skill", "/v1/skills/assess", `{"person_id":"EMP-0030","skill_code":"FORK","level":4,"assessed_at":"2026-11-18","valid_until":"2027-11-18"}`, 201},
		{"training", "/v1/training/records", `{"person_id":"EMP-0030","program_id":"TP-PCCC","completed_on":"2026-11-18","result":"passed","valid_until":"2027-11-18"}`, 201},
		{"kpi", "/v1/performance", `{"person_id":"EMP-0030","period":"2026-10","metric_code":"SAFETY","actual":107,"target":100}`, 201},
		{"absence", "/v1/absences", `{"person_id":"EMP-0030","type":"annual","start":"2026-11-20","end":"2026-11-22"}`, 201},
		{"recognition", "/v1/recognitions", `{"person_id":"EMP-0030","category":"Kaizen safety","date":"2026-11-18"}`, 201},
		{"discipline", "/v1/discipline", `{"person_id":"EMP-0030","date":"2026-11-18"}`, 201},
		{"workflow", "/v1/workflow/requests", `{"subject_id":"EMP-0030","type":"training","summary":"Request training course","effective_date":"2026-11-18"}`, 201},
		{"benefit", "/v1/benefits/claims", `{"person_id":"EMP-0030","type":"sickness","date":"2026-11-18"}`, 201},
	}
	ids := map[string]string{}
	for _, x := range cases {
		t.Run(x.name, func(t *testing.T) {
			status, b := serve(t, s, "POST", x.path, x.data, true)
			if status != x.want {
				t.Fatalf("status %d: %s", status, string(b))
			}
			ids[x.name] = obj(t, b)["id"].(string)
		})
	}
	for _, x := range []struct{ name, path, data string }{
		{"absence approval", "/v1/absences/" + ids["absence"] + "/decision", `{"status":"approved"}`},
		{"workflow approval", "/v1/workflow/requests/" + ids["workflow"] + "/decision", `{"status":"approved","note":"Local test"}`},
	} {
		t.Run(x.name, func(t *testing.T) {
			status, b := serve(t, s, "POST", x.path, x.data, true)
			if status != 201 {
				t.Fatalf("status %d: %s", status, string(b))
			}
		})
	}
	status, b := serve(t, s, "POST", "/v1/absences/"+ids["absence"]+"/decision", `{"status":"rejected"}`, true)
	if status != 409 {
		t.Fatalf("already decided absence should conflict: %d %s", status, b)
	}
	cnt, e := count(s, "SELECT COUNT(*) AS n FROM audit_log WHERE tenant_id=?", tenantID)
	if e != nil || cnt < 10 {
		t.Fatalf("audit should track writes, got %d %v", cnt, e)
	}
}
func TestTemporalReportingAndGuardrails(t *testing.T) {
	s := testService(t)
	for _, p := range []string{"2026-02-01", "2026-11-18", "2027-01-01"} {
		status, b := serve(t, s, "GET", "/v1/people/EMP-0030/manager?as_of="+p, "", true)
		if status != 200 {
			t.Fatalf("%s: %d %s", p, status, b)
		}
	}
	a := obj(t, mustGet(t, s, "/v1/people/EMP-0030/manager?as_of=2026-02-01"))
	b := obj(t, mustGet(t, s, "/v1/people/EMP-0030/manager?as_of=2026-11-18"))
	aa, _ := json.Marshal(a["manager"])
	bb, _ := json.Marshal(b["manager"])
	if string(aa) == string(bb) {
		t.Fatal("historical manager lookup should change across seeded transfers")
	}
	for _, v := range []struct{ path, data string }{
		{"/v1/skills/assess", `{"person_id":"EMP-0030","skill_code":"FORK","level":5,"assessed_at":"2026-11-18"}`},
		{"/v1/performance", `{"person_id":"EMP-0030","period":"invalid","metric_code":"QUALITY","actual":100,"target":100}`},
		{"/v1/absences", `{"person_id":"EMP-0030","type":"annual","start":"2026-12-01","end":"2026-11-30"}`},
	} {
		status, body := serve(t, s, "POST", v.path, v.data, true)
		if status != 422 {
			t.Fatalf("expected 422 %s: %d %s", v.path, status, body)
		}
	}
	status, body := serve(t, s, "GET", "/v1/people/search", "", false)
	if status != 401 {
		t.Fatalf("unauthed request allowed: %d %s", status, body)
	}
}
func mustGet(t *testing.T, s *Service, path string) []byte {
	t.Helper()
	status, body := serve(t, s, "GET", path, "", true)
	if status != 200 {
		t.Fatal(fmt.Sprintf("HTTP %d %s", status, body))
	}
	return body
}
func TestNewWorkspaceServesAssets(t *testing.T) {
	s := testService(t)
	for _, p := range []string{"/assets/app.js", "/assets/app.css", "/app", "/"} {
		status, b := serve(t, s, "GET", p, "", false)
		if status != 200 {
			t.Fatalf("%s status %d", p, status)
		}
		if len(b) < 500 {
			t.Fatalf("%s content too short", p)
		}
	}
	status, b := serve(t, s, "GET", "/app", "", false)
	if status != 200 || !strings.Contains(string(b), "WORKFORCE INTELLIGENCE") {
		t.Fatal("workspace HTML missing navigation shell")
	}
}
func TestWorkforceLifecycleAndEligibility(t *testing.T) {
	s := testService(t)
	status, b := serve(t, s, "GET", "/v1/workforce/eligibility?person_id=EMP-0030&task_code=FORK&as_of=2026-11-18", "", true)
	if status != 200 {
		t.Fatalf("eligibility: %d %s", status, b)
	}
	elig := obj(t, b)
	if elig["authorized_to_work"] != false {
		t.Fatalf("skill-only engine must not grant permit: %s", b)
	}
	status, b = serve(t, s, "POST", "/v1/people", `{"name":"Nguyễn Văn Demo","position_code":"WH-OP","effective_date":"2026-12-01"}`, true)
	if status != 201 {
		t.Fatalf("hire %d %s", status, b)
	}
	id := obj(t, b)["person_id"].(string)
	status, b = serve(t, s, "GET", "/v1/people/detail/"+id+"?as_of=2026-12-02", "", true)
	if status != 200 {
		t.Fatalf("new employee should be found %d %s", status, b)
	}
	status, b = serve(t, s, "POST", "/v1/people/"+id+"/separation", `{"effective_date":"2027-01-01","reason":"resignation"}`, true)
	if status != 201 {
		t.Fatalf("separation %d %s", status, b)
	}
	status, b = serve(t, s, "GET", "/v1/people/"+id+"/manager?as_of=2027-01-02", "", true)
	if status != 200 || obj(t, b)["status"] != "unassigned" {
		t.Fatalf("separated employee must be unassigned: %d %s", status, b)
	}
}
