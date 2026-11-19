//go:build linux && cgo

package main

import (
    "encoding/json"
    "testing"
)

func TestKaizenExternalLaunchPolicy(t *testing.T) {
    t.Setenv("ORGAXIS_KAIZEN_URL", "")
    missing,err:=kaizenLaunchV12()
    if err!=nil||missing.(map[string]any)["status"]!="NOT_CONFIGURED" {t.Fatalf("empty config: %v %v",missing,err)}
    t.Setenv("ORGAXIS_KAIZEN_URL","https://kaizen.example.test")
    blocked,_:=kaizenLaunchV12()
    if blocked.(map[string]any)["status"]!="BLOCKED_PREREQUISITES" {t.Fatalf("unapproved launch was allowed: %v",blocked)}
    if !kaizenExternalConfiguredV12() {t.Fatal("legacy BONUS writes must be fenced while URL awaits approvals")}
    t.Setenv("ORGAXIS_P0_ORG_APPROVAL_REF","HR-ORG-TEST-001")
    t.Setenv("ORGAXIS_P0_PERSON_APPROVAL_REF","HR-PEOPLE-TEST-001")
    t.Setenv("ORGAXIS_P0_KAIZEN_APPROVAL_REF","MIS-FINANCE-TEST-001")
    valid:=[]string{"https://kaizen.example.test/", "http://127.0.0.1:8080", "http://localhost:8080"}
    for _,endpoint:=range valid {
        t.Setenv("ORGAXIS_KAIZEN_URL",endpoint)
        data,e:=kaizenLaunchV12()
        if e!=nil||data.(map[string]any)["status"]!="CONFIGURED_UNVERIFIED" {t.Fatalf("valid endpoint %s: %v %v",endpoint,data,e)}
        if data.(map[string]any)["auth"]!="SEPARATE_LOGIN" {t.Fatal("no cross-app session sharing allowed")}
    }
    invalidURLs:=[]string{"http://192.168.1.20:8080","http://kaizen.example.test/","javascript:alert(1)","https://user:pass@kaizen.example.test", "https://kaizen.example.test/?token=secret","https://kaizen.example.test/#token"}
    for _,endpoint:=range invalidURLs {
        t.Setenv("ORGAXIS_KAIZEN_URL",endpoint)
        if data,e:=kaizenLaunchV12();e==nil {t.Fatalf("unsafe URL accepted %s: %v",endpoint,data)}
    }
}

func TestKaizenLaunchAuthenticatedAndLegacyBonusGuard(t *testing.T) {
    v:=secured(t)
    t.Setenv("ORGAXIS_KAIZEN_URL","https://kaizen.example.test/")
    t.Setenv("ORGAXIS_P0_ORG_APPROVAL_REF","HR-ORG-TEST-001")
    t.Setenv("ORGAXIS_P0_PERSON_APPROVAL_REF","HR-PEOPLE-TEST-001")
    t.Setenv("ORGAXIS_P0_KAIZEN_APPROVAL_REF","MIS-FINANCE-TEST-001")
    if w:=call(v.service,"GET","/v2/kaizen/launch","",nil,false);w.Code!=401 {t.Fatalf("anonymous launch endpoint: %d",w.Code)}
    employee:=signin(t,v,"employee")
    w:=call(v.service,"GET","/v2/kaizen/launch","",employee,false)
    if w.Code!=200 {t.Fatalf("signed-in launch: %d %s",w.Code,w.Body.String())}
    var data map[string]any
    if err:=json.Unmarshal(w.Body.Bytes(),&data);err!=nil {t.Fatal(err)}
    if data["status"]!="CONFIGURED_UNVERIFIED"||data["production_data"]!="NOT_VERIFIED" {t.Fatalf("unsafe launch claim: %v",data)}
    // Once the external Kaizen link is enabled, v0.6 BONUS must not
    // silently create a second initiative/reward ledger.
    w=call(v.service,"POST","/v2/cases",`{"type":"BONUS","subject_id":"EMP-0012","title":"duplicate"}`,employee,true)
    if w.Code!=409 {t.Fatalf("duplicate BONUS ledger not blocked: %d %s",w.Code,w.Body.String())}
}
