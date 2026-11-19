package main

import (
    "net"
    "net/http"
    "net/url"
    "os"
    "strings"
)

// kaizenLaunchV12 gives users a navigational entry point into the independently
// authenticated Kaizen service. It does NOT proxy sessions or send any person data.
// OrgAxis and Kaizen retain independent users/roles until verified HR source
// integration is approved. The service is not asserted production-ready.
func kaizenLaunchV12() (any,error) {
    raw:=strings.TrimSpace(os.Getenv("ORGAXIS_KAIZEN_URL"))
    if raw=="" {
        return map[string]any{
            "status":"NOT_CONFIGURED",
            "source":"ngtrthanh/chinfon-kaizen",
            "auth":"SEPARATE_LOGIN",
            "production_data":"NOT_VERIFIED",
            "warning":"Configure a reviewed HTTPS Kaizen endpoint after the production gates have passed.",
        },nil
    }
    u,e:=url.Parse(raw)
    if e!=nil || u==nil || u.Hostname()=="" || u.User!=nil || u.RawQuery!="" || u.Fragment!="" {
        return nil,fail(http.StatusUnprocessableEntity,"Invalid Kaizen module endpoint")
    }
    scheme:=strings.ToLower(u.Scheme)
    host:=strings.ToLower(u.Hostname())
    loopback:=host=="localhost"
    if ip:=net.ParseIP(host);ip!=nil && ip.IsLoopback(){loopback=true}
    if scheme!="https" && !(scheme=="http" && loopback){
        return nil,fail(http.StatusUnprocessableEntity,"Kaizen module requires HTTPS or local loopback")
    }
    // The URL by itself must not activate a production-looking reward module.
    // References are configuration assertions, NOT independently verified evidence.
    required:=[]struct{key,label string}{
        {"ORGAXIS_P0_ORG_APPROVAL_REF","ORG-03 effective reporting approved by HR"},
        {"ORGAXIS_P0_PERSON_APPROVAL_REF","PERSON-01 employee master approved by HR"},
        {"ORGAXIS_P0_KAIZEN_APPROVAL_REF","Kaizen MIS/Finance UAT and security approval"},
    }
    missing:=make([]string,0)
    for _,item:=range required{
        if len(strings.TrimSpace(os.Getenv(item.key)))<6{missing=append(missing,item.label)}
    }
    if len(missing)>0 {
        return map[string]any{
            "status":"BLOCKED_PREREQUISITES",
            "source":"ngtrthanh/chinfon-kaizen",
            "auth":"SEPARATE_LOGIN",
            "production_data":"NOT_VERIFIED",
            "missing":missing,
            "warning":"HTTPS endpoint is configured but real ORG-03, PERSON-01, MIS/Finance approvals are missing. No launch link returned.",
        },nil
    }
    return map[string]any{
        "status":"CONFIGURED_UNVERIFIED",
        "url":u.String(),
        "source":"ngtrthanh/chinfon-kaizen",
        "auth":"SEPARATE_LOGIN",
        "production_data":"NOT_VERIFIED",
        "warning":"Approval refs are operator-declared, not independently verified by OrgAxis. Validate signed source documents before accepting production.",
    },nil
}

func kaizenExternalConfiguredV12() bool {
    info,e:=kaizenLaunchV12()
    if e!=nil{return false}
    m,ok:=info.(map[string]any)
    return ok && (m["status"]=="CONFIGURED_UNVERIFIED" || m["status"]=="BLOCKED_PREREQUISITES")
}
