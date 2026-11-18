package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RotatingLog keeps a bounded number of logs. Logs are deliberately local-only.
type RotatingLog struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
	max  int64
	keep int
}

func NewRotatingLog(path string, maxBytes int64, keep int) (*RotatingLog, error) {
	l := &RotatingLog{path: path, max: maxBytes, keep: keep}
	if err := l.reopen(); err != nil {
		return nil, err
	}
	return l, nil
}
func (l *RotatingLog) reopen() error {
	f, e := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	s, e := f.Stat()
	if e != nil {
		f.Close()
		return e
	}
	l.file = f
	l.size = s.Size()
	return nil
}
func (l *RotatingLog) rotate() error {
	if l.file != nil {
		if e := l.file.Close(); e != nil {
			return e
		}
		l.file = nil
	}
	for i := l.keep - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", l.path, i)
		next := fmt.Sprintf("%s.%d", l.path, i+1)
		if i == l.keep-1 {
			_ = os.Remove(next)
		}
		if _, e := os.Stat(old); e == nil {
			if e := os.Rename(old, next); e != nil {
				return e
			}
		}
	}
	if _, e := os.Stat(l.path); e == nil {
		if e := os.Rename(l.path, l.path+".1"); e != nil {
			return e
		}
	}
	return l.reopen()
}
func (l *RotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(p)) > l.max {
		if e := l.rotate(); e != nil {
			return 0, e
		}
	}
	n, e := l.file.Write(p)
	l.size += int64(n)
	return n, e
}
func (l *RotatingLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
func (l *RotatingLog) Tail(lines int) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lines < 1 || lines > 500 {
		return nil, fmt.Errorf("lines out of range")
	}
	f, e := os.Open(l.path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	size := st.Size()
	start := size - int64(256*1024)
	if start < 0 {
		start = 0
	}
	if _, e = f.Seek(start, 0); e != nil {
		return nil, e
	}
	b, e := io.ReadAll(io.LimitReader(f, 256*1024))
	if e != nil {
		return nil, e
	}
	ss := strings.Split(string(b), "\n")
	if start > 0 && len(ss) > 0 {
		ss = ss[1:]
	}
	if len(ss) > 0 && ss[len(ss)-1] == "" {
		ss = ss[:len(ss)-1]
	}
	if len(ss) > lines {
		ss = ss[len(ss)-lines:]
	}
	return ss, nil
}

// LAN mode is a separate read-only TLS listener. It never serves the full HR UI or writes.
var lanPaths = map[string]bool{
	"/v1/org/blank": true, "/v1/org/occupied": true,
	"/v1/workflow/resolve": true,
}

func lanAllowed(p string) bool {
	if lanPaths[p] {
		return true
	}
	if strings.HasPrefix(p, "/v1/people/") {
		parts := strings.Split(strings.Trim(p, "/"), "/")
		return len(parts) == 4 && parts[1] == "people" && (parts[3] == "manager" || parts[3] == "chain" || parts[3] == "reports")
	}
	return false
}
func lanHandler(local http.Handler, token string, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" || !lanAllowed(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		// Use Authorization: Bearer only; demo local API key is never accepted on LAN.
		auth := r.Header.Get("Authorization")
		supplied := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") || len(supplied) != len(token) || !constantTimeEqual(supplied, token) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="OrgAxis LAN read-only"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		rr := r.Clone(r.Context())
		rr.Header = rr.Header.Clone()
		// This trusted internal header is created only after separate LAN token validation.
		rr.Header.Set("X-API-Key", apiKey)
		local.ServeHTTP(w, rr)
	})
}
func constantTimeEqual(a, b string) bool {
	h1 := sha256.Sum256([]byte(a))
	h2 := sha256.Sum256([]byte(b))
	var x byte
	for i := range h1 {
		x |= h1[i] ^ h2[i]
	}
	return x == 0
}

type LanCredentials struct{ CertFile, KeyFile, TokenFile, Token, Fingerprint string }

func generateLANCredentials(dir string, ip net.IP) (LanCredentials, error) {
	p := filepath.Join(dir, "lan")
	if e := os.MkdirAll(p, 0700); e != nil {
		return LanCredentials{}, e
	}
	certFile, keyFile, tokenFile := filepath.Join(p, "cert.pem"), filepath.Join(p, "key.pem"), filepath.Join(p, "api-token.txt")
	// Reusing an existing certificate is allowed only if the requested LAN IP is in its SANs.
	if cb, e := os.ReadFile(certFile); e == nil {
		kb, e1 := os.ReadFile(keyFile)
		tb, e2 := os.ReadFile(tokenFile)
		pair, e3 := tls.X509KeyPair(cb, kb)
		if e1 != nil || e2 != nil || e3 != nil {
			return LanCredentials{}, fmt.Errorf("incomplete LAN TLS credentials: %v / %v / %v", e1, e2, e3)
		}
		cert, e4 := x509.ParseCertificate(pair.Certificate[0])
		if e4 != nil {
			return LanCredentials{}, e4
		}
		validIP := false
		for _, v := range cert.IPAddresses {
			if v.Equal(ip) {
				validIP = true
			}
		}
		if !validIP {
			return LanCredentials{}, fmt.Errorf("existing LAN certificate is for a different IP; regenerate it explicitly")
		}
		if time.Now().After(cert.NotAfter) {
			return LanCredentials{}, fmt.Errorf("LAN TLS certificate has expired; regenerate it explicitly")
		}
		token := strings.TrimSpace(string(tb))
		if len(token) < 64 {
			return LanCredentials{}, fmt.Errorf("invalid stored LAN bearer token")
		}
		sum := sha256.Sum256(cert.Raw)
		return LanCredentials{certFile, keyFile, tokenFile, token, hex.EncodeToString(sum[:])}, nil
	}
	if _, e := os.Stat(keyFile); e == nil {
		return LanCredentials{}, fmt.Errorf("LAN private key exists but certificate is missing; manual intervention required")
	}
	if _, e := os.Stat(tokenFile); e == nil {
		return LanCredentials{}, fmt.Errorf("LAN bearer token exists but certificate is missing; manual intervention required")
	}
	key, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		return LanCredentials{}, e
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return LanCredentials{}, e
	}
	cert := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "OrgAxis LAN Demo"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IPAddresses: []net.IP{ip}}
	der, e := x509.CreateCertificate(rand.Reader, &cert, &cert, &key.PublicKey, key)
	if e != nil {
		return LanCredentials{}, e
	}
	pk := x509.MarshalPKCS1PrivateKey(key)
	if e = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pk}), 0600); e != nil {
		return LanCredentials{}, e
	}
	if e = os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); e != nil {
		return LanCredentials{}, e
	}
	tok := make([]byte, 32)
	if _, e = rand.Read(tok); e != nil {
		return LanCredentials{}, e
	}
	token := hex.EncodeToString(tok)
	if e = os.WriteFile(tokenFile, []byte(token+"\n"), 0600); e != nil {
		return LanCredentials{}, e
	}
	sum := sha256.Sum256(der)
	_ = os.WriteFile(filepath.Join(p, "README.txt"), []byte("ORGAXIS LAN DEMO ONLY\nTLS self-signed certificate: cert.pem\nBearer credential: api-token.txt (keep private)\nConfigure clients to verify cert.pem fingerprint. Do not disable certificate verification for real data.\n"), 0600)
	return LanCredentials{certFile, keyFile, tokenFile, token, hex.EncodeToString(sum[:])}, nil
}
func validateLANHost(host string) (net.IP, error) {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsPrivate() {
		return nil, fmt.Errorf("LAN host must be a private IP bound to this machine, e.g. 192.168.11.11")
	}
	return ip, nil
}
func newLANServer(local http.Handler, dir, host string, port int, apiKey string) (*http.Server, net.Listener, LanCredentials, error) {
	ip, e := validateLANHost(host)
	if e != nil {
		return nil, nil, LanCredentials{}, e
	}
	if port < 1 || port > 65535 {
		return nil, nil, LanCredentials{}, fmt.Errorf("invalid LAN port")
	}
	cred, e := generateLANCredentials(dir, ip)
	if e != nil {
		return nil, nil, LanCredentials{}, e
	}
	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		return nil, nil, LanCredentials{}, e
	}
	s := &http.Server{Addr: addr, Handler: lanHandler(local, cred.Token, apiKey), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 1 << 16}
	// TLS config with a minimum of TLS 1.2; cert validation remains client responsibility.
	pair, e := tls.LoadX509KeyPair(cred.CertFile, cred.KeyFile)
	if e != nil {
		ln.Close()
		return nil, nil, LanCredentials{}, e
	}
	s.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	return s, ln, cred, nil
}

func logRuntimeInfo(dataPath string) { log.Printf("OrgAxis v0.5 starting; data=%s", dataPath) }
