//go:build linux && cgo

package main

import (
  "encoding/json"
  "testing"
)

func TestV11ModuleCatalogAuthAndCoverage(t *testing.T) {
  v:=secured(t)
  unauth:=call(v.service,"GET","/v2/modules","",nil,false)
  if unauth.Code!=401 {t.Fatalf("unauthed catalog returned %d",unauth.Code)}
  employee:=signin(t,v,"employee")
  if denied:=call(v.service,"GET","/v2/modules","",employee,false);denied.Code!=403 {t.Fatalf("employee inventory scope: %d",denied.Code)}
  admin:=signin(t,v,"admin")
  x:=call(v.service,"GET","/v2/modules","",admin,false)
  if x.Code!=200 {t.Fatalf("catalog: %d %s",x.Code,x.Body.String())}
  data:=obj(t,x.Body.Bytes())
  if data["appgate_assessment"]!="NOT_ASSESSED" || data["catalog_type"]!="source_review" {
    t.Fatalf("untrue assurance claims: %v",data)
  }
  n:=int(data["count"].(float64))
  if n!=len(modulesV11) || n<30 {t.Fatalf("incomplete module registry: %d vs %d",n,len(modulesV11))}
  ids:=map[string]bool{}
  statuses:=map[string]bool{"operational_demo":true,"partial_demo":true,"design_only":true,"not_implemented":true}
  for _,it:=range data["modules"].([]any) {
    m:=it.(map[string]any)
    id:=m["id"].(string)
    if ids[id] {t.Fatalf("duplicated module id %s",id)}
    ids[id]=true
    if m["maturity"].(float64)<0 || m["maturity"].(float64)>5 {t.Fatalf("invalid maturity %s",id)}
    if !statuses[m["status"].(string)] {t.Fatalf("invalid status %s",id)}
    if len(m["functions"].([]any))==0 || len(m["evidence"].([]any))==0 || len(m["gaps"].([]any))==0 {
      t.Fatalf("missing function/evidence/gap %s",id)
    }
  }
  for _,id:=range []string{"ORG-02","ORG-05","ORG-06","WFM-04","WFM-06","PAY-02","INT-02","GATE-01"} {
    if !ids[id] {t.Fatalf("key module missing: %s",id)}
  }
}

func TestV11CatalogFiltersAndNoFalsePromotion(t *testing.T) {
  v:=secured(t)
  admin:=signin(t,v,"admin")
  for _,path:=range []string{"/v2/modules?group=BAD","/v2/modules?status=uat_pass"} {
    x:=call(v.service,"GET",path,"",admin,false)
    if x.Code!=422 {t.Fatalf("%s accepted: %d",path,x.Code)}
  }
  x:=call(v.service,"GET","/v2/modules?group=WFM&status=design_only","",admin,false)
  if x.Code!=200 {t.Fatalf("filter: %s",x.Body)}
  var payload struct {Count int `json:"count"`;Modules []moduleV11 `json:"modules"`}
  if e:=json.Unmarshal(x.Body.Bytes(),&payload);e!=nil {t.Fatal(e)}
  if payload.Count==0 || len(payload.Modules)!=payload.Count {t.Fatalf("bad filtered count: %+v",payload)}
  for _,m:=range payload.Modules {
    if m.Group!="WFM" || m.Status!="design_only" {t.Fatalf("wrong filter result: %+v",m)}
  }
  for _,m:=range modulesV11 {
    if (m.ID=="ORG-05"||m.ID=="ORG-06"||m.ID=="WFM-06") && (m.Status!="design_only"||m.Maturity>1) {
      t.Fatalf("spec-only falsely promoted: %+v",m)
    }
    if m.ID=="PAY-02" && m.Maturity>=2 {t.Fatal("legal payroll falsely promoted")}
  }
}
