//go:build linux && cgo

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBoundedLogAndReadAPI(t *testing.T) {
	s := testService(t)
	path := filepath.Join(t.TempDir(), "orgaxis.log")
	l, e := NewRotatingLog(path, 512, 2)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	s.logfile = l
	s.started = time.Now().Add(-3 * time.Minute)
	for i := 0; i < 120; i++ {
		_, e = l.Write([]byte(fmt.Sprintf("record %03d lorem ipsum\n", i)))
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range []string{"/logs", "/v1/system/status", "/v1/system/logs?lines=30"} {
		status, b := serve(t, s, "GET", p, "", p != "/logs")
		if status != 200 || len(b) < 5 {
			t.Fatalf("%s status=%d body=%s", p, status, b)
		}
	}
	status, _ := serve(t, s, "GET", "/v1/system/logs?lines=501", "", true)
	if status != 422 {
		t.Fatal(status)
	}
	lines, e := l.Tail(30)
	if e != nil || len(lines) == 0 || len(lines) > 30 {
		t.Fatalf("log lines %d %v", len(lines), e)
	}
	if _, e := os.Stat(path + ".1"); e != nil {
		t.Fatal("log rotation missing", e)
	}
}
func TestLANReadOnlyAndSeparateBearer(t *testing.T) {
	s := testService(t)
	protected := lanHandler(s.routes(), strings.Repeat("a", 64), "demo-local-key")
	cases := []struct {
		method, path, token string
		status              int
	}{
		{"GET", "/v1/org/blank?as_of=2026-11-18", "", 401},
		{"GET", "/v1/org/blank?as_of=2026-11-18", "demo-local-key", 401},
		{"GET", "/v1/org/blank?as_of=2026-11-18", strings.Repeat("a", 64), 200},
		{"GET", "/v1/people/EMP-0030/manager?as_of=2026-11-18", strings.Repeat("a", 64), 200},
		{"GET", "/v1/payroll/workbench?period=2026-10", strings.Repeat("a", 64), 404},
		{"GET", "/v1/system/logs", strings.Repeat("a", 64), 404},
		{"GET", "/app", strings.Repeat("a", 64), 404},
		{"POST", "/v1/companies", strings.Repeat("a", 64), 404},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(nil))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("X-API-Key", "demo-local-key")
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Errorf("%s %s got %d want %d: %s", tc.method, tc.path, w.Code, tc.status, w.Body.String())
		}
	}
}
func TestLANCertificateAndTLS(t *testing.T) {
	dir := t.TempDir()
	ip := net.ParseIP("192.168.11.11")
	a, e := generateLANCredentials(dir, ip)
	if e != nil {
		t.Fatal(e)
	}
	b, e := generateLANCredentials(dir, ip)
	if e != nil {
		t.Fatal(e)
	}
	if a.Token != b.Token || a.Fingerprint != b.Fingerprint {
		t.Fatal("credentials regenerated unexpectedly")
	}
	if _, e := generateLANCredentials(dir, net.ParseIP("192.168.11.12")); e == nil {
		t.Fatal("wrong IP not detected")
	}
	certPem, e := os.ReadFile(a.CertFile)
	if e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPem) {
		t.Fatal("cert parse")
	}
	cert, e := tls.LoadX509KeyPair(a.CertFile, a.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	tlsSrv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	tlsSrv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	tlsSrv.StartTLS()
	defer tlsSrv.Close()
	// The test server binds 127.0.0.1; cert SAN is 192.168.11.11, so test certificate matching explicitly.
	if e = cert.Leaf.VerifyHostname("192.168.11.11"); e != nil {
		t.Fatal(e)
	}
	if len(pool.Subjects()) == 0 {
		t.Fatal("certificate pool empty")
	}
}
func TestSingleWriterLock(t *testing.T) {
	dir := t.TempDir()
	u, e := acquireInstance(dir)
	if e != nil {
		t.Fatal(e)
	}
	_, e = acquireInstance(dir)
	if e == nil {
		t.Fatal("second process lock unexpectedly succeeded")
	}
	u()
	v, e := acquireInstance(dir)
	if e != nil {
		t.Fatal("lock not released", e)
	}
	v()
}
func TestCancelServerMode(t *testing.T) {
	// Exercise a real local HTTP server on a random high port and verify durable shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	c := Config{Mode: "server", Port: port, Data: filepath.Join(t.TempDir(), "data"), APIKey: "demo-local-key", NoBrowser: true}
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- runApp(ctx, c, func() { close(ready) }) }()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("server failed to start")
	}
	resp, e := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if e != nil {
		t.Fatal(e)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server failed to stop")
	}
}
