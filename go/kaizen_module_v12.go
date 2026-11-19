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
    return map[string]any{
        "status":"CONFIGURED_UNVERIFIED",
        "url":u.String(),
        "source":"ngtrthanh/chinfon-kaizen",
        "auth":"SEPARATE_LOGIN",
        "production_data":"NOT_VERIFIED",
        "warning":"Configured link only. Confirm Kaizen TLS, independent login, real HR data, approval controls and MIS UAT before production.",
    },nil
}

func kaizenExternalConfiguredV12() bool {
    info,e:=kaizenLaunchV12()
    if e!=nil{return false}
    m,ok:=info.(map[string]any)
    return ok && m["status"]=="CONFIGURED_UNVERIFIED"
}
