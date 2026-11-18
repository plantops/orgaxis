package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	Mode      string
	Port      int
	Data      string
	APIKey    string
	NoBrowser bool
	LANHost   string
	LANPort   int
}

func runMain() error {
	c := Config{}
	flag.StringVar(&c.Mode, "mode", "desktop", "desktop (tray + API), client (tray only), server (headless), service (Windows SCM)")
	flag.IntVar(&c.Port, "port", 8765, "Loopback HTTP port")
	flag.StringVar(&c.Data, "data", "", "Data directory (default ./data next to EXE)")
	flag.StringVar(&c.APIKey, "api-key", "", "Local demo API key override")
	flag.BoolVar(&c.NoBrowser, "no-browser", false, "Do not open the browser")
	flag.StringVar(&c.LANHost, "lan-host", "", "Explicit private LAN IP for read-only TLS API (disabled by default)")
	flag.IntVar(&c.LANPort, "lan-port", 8766, "TLS LAN API port")
	legacyTray := flag.Bool("tray", true, "(compatibility) Use tray in desktop mode")
	flag.Parse()
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	if c.LANPort == c.Port && c.LANHost != "" {
		return fmt.Errorf("LAN and localhost listeners must have different ports")
	}
	if c.Data == "" {
		ex, e := os.Executable()
		if e != nil {
			return e
		}
		c.Data = filepath.Join(filepath.Dir(ex), "data")
	}
	c.Data, _ = filepath.Abs(c.Data)
	if c.APIKey == "" {
		c.APIKey = os.Getenv("ORGAXIS_API_KEY")
	}
	if c.APIKey == "" {
		c.APIKey = "demo-local-key"
	}
	if !*legacyTray && c.Mode == "desktop" {
		c.Mode = "server"
	}
	switch c.Mode {
	case "desktop", "server", "service", "client":
	default:
		return fmt.Errorf("invalid mode %s", c.Mode)
	}
	if c.Mode == "client" {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("tray client requires Windows")
		}
		url := fmt.Sprintf("http://127.0.0.1:%d/", c.Port)
		if !c.NoBrowser {
			launchBrowser(url)
		}
		return runWindowsTray(url, filepath.Join(c.Data, "orgaxis-tray.ico"), nil, "client")
	}
	if c.Mode == "service" {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("Windows service mode is Windows-only")
		}
		return runWindowsService(func(ctx context.Context, ready func()) error { return runApp(ctx, c, ready) })
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runApp(ctx, c, nil)
}

func runApp(ctx context.Context, c Config, ready func()) error {
	if e := os.MkdirAll(c.Data, 0700); e != nil {
		return e
	}
	// Mutual exclusion is keyed by canonical database folder, across desktop and service modes.
	unlock, e := acquireInstance(c.Data)
	if e != nil {
		if c.Mode == "desktop" {
			launchBrowser(fmt.Sprintf("http://127.0.0.1:%d/", c.Port))
		}
		return e
	}
	defer unlock()
	logfile, e := NewRotatingLog(filepath.Join(c.Data, "orgaxis.log"), 10<<20, 4)
	if e != nil {
		return e
	}
	defer func() { log.SetOutput(os.Stderr); _ = logfile.Close() }()
	log.SetOutput(logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lmicroseconds)
	logRuntimeInfo(c.Data)
	dbPath, e := setupDB(c.Data)
	if e != nil {
		return e
	}
	db, e := openSQLite(dbPath)
	if e != nil {
		return e
	}
	defer db.Close()
	if backup, be := backupBeforeUpgrade(db, dbPath); be != nil {
		return be
	} else if backup != "" {
		log.Printf("Backed up database: %s", backup)
	}
	if e = enhanceDB(db); e != nil {
		return e
	}
	if backup, be := backupBeforeWorkbench(db, dbPath); be != nil {
		return be
	} else if backup != "" {
		log.Printf("Backed up payroll database: %s", backup)
	}
	if e = upgradePayrollWorkbench(db); e != nil {
		return e
	}
	if backup, e2 := backupBeforeSecurity(db, dbPath); e2 != nil {
		return e2
	} else if backup != "" {
		log.Printf("Backup before secure v0.6 migration: %s", backup)
	}
	if e = secureSchema(db); e != nil {
		return e
	}
	if backup, be := backupBeforeAttendance(db, dbPath); be != nil {
		return be
	} else if backup != "" {
		log.Printf("Backup before v0.7 time schema: %s", backup)
	}
	if e = attendanceSchema(db); e != nil {
		return e
	}
	if backup, be := backupBeforeCheckpoint(db, dbPath); be != nil {
		return be
	} else if backup != "" {
		log.Printf("Backup before v0.8 device checkpoint migration: %s", backup)
	}
	if e = attendanceCheckpointSchema(db); e != nil {
		return e
	}
	if e = bootstrapUsers(db, c.Data); e != nil {
		return e
	}
	if e = seedAttendanceDemo(db); e != nil {
		return e
	}
	if e = seedV6Cases(db); e != nil {
		return e
	}
	s := &Service{db: db, key: c.APIKey, port: c.Port, logfile: logfile, started: time.Now(), secureDemo: true}
	handler := s.routes()
	localListener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.Port))
	if e != nil {
		return fmt.Errorf("localhost listener: %w", e)
	}
	defer localListener.Close()
	localServer := httpServer(handler)
	results := make(chan error, 2)
	var lanServer *http.Server
	var lanListener net.Listener
	if c.LANHost != "" {

		l, ln, cred, ee := newLANServer(handler, c.Data, c.LANHost, c.LANPort, c.APIKey)
		if ee != nil {
			return ee
		}
		lanServer = l
		lanListener = ln
		log.Printf("LAN read-only TLS listener https://%s:%d; certificate SHA256=%s; token file=%s", c.LANHost, c.LANPort, cred.Fingerprint, cred.TokenFile)
	}
	go func() {
		if e := localServer.Serve(localListener); e != nil && !errors.Is(e, http.ErrServerClosed) {
			results <- fmt.Errorf("local server: %w", e)
		}
	}()
	if lanServer != nil {
		go func() {
			if e := lanServer.ServeTLS(lanListener, "", ""); e != nil && !errors.Is(e, http.ErrServerClosed) {
				results <- fmt.Errorf("LAN TLS server: %w", e)
			}
		}()
	}
	log.Printf("OrgAxis local HTTP active: http://127.0.0.1:%d/ mode=%s", c.Port, c.Mode)
	if ready != nil {
		ready()
	}
	if c.Mode == "desktop" && runtime.GOOS == "windows" {
		if !c.NoBrowser {
			launchBrowser(fmt.Sprintf("http://127.0.0.1:%d/", c.Port))
		}
		trayErr := make(chan error, 1)
		go func() {
			trayErr <- runWindowsTray(fmt.Sprintf("http://127.0.0.1:%d/", c.Port), filepath.Join(c.Data, "orgaxis-tray.ico"), nil, "desktop")
		}()
		select {
		case <-ctx.Done():
		case e = <-trayErr:
			if e != nil {
				log.Printf("Tray failure: %v", e)
				return e
			}
		case e = <-results:
		}
	} else {
		if c.Mode == "server" && !c.NoBrowser && runtime.GOOS == "windows" {
			launchBrowser(fmt.Sprintf("http://127.0.0.1:%d/", c.Port))
		}
		select {
		case <-ctx.Done():
		case e = <-results:
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if lanServer != nil {
		if ee := lanServer.Shutdown(shutdownCtx); ee != nil {
			log.Printf("LAN shutdown: %v", ee)
		}
		if lanListener != nil {
			lanListener.Close()
		}
	}
	if ee := localServer.Shutdown(shutdownCtx); ee != nil {
		log.Printf("local shutdown: %v", ee)
	}
	log.Printf("OrgAxis stopped mode=%s", c.Mode)
	if e != nil && !errors.Is(e, http.ErrServerClosed) {
		return e
	}
	return nil
}
func httpServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 1 << 20}
}

func startupModeCommand(mode string) string {
	exe, e := os.Executable()
	if e != nil {
		return ""
	}
	return `"` + strings.ReplaceAll(exe, `"`, "") + `" -mode ` + mode + ` -no-browser`
}
func main() {
	if e := runMain(); e != nil {
		log.Printf("OrgAxis startup failed: %v", e)
		if runtime.GOOS == "windows" && !strings.Contains(strings.Join(os.Args, " "), "-mode service") && !strings.Contains(strings.Join(os.Args, " "), "-mode=service") {
			showStartupError(e.Error())
		} else {
			fmt.Fprintln(os.Stderr, "OrgAxis:", e)
		}
		os.Exit(1)
	}
}
