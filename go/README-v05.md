# OrgAxis v0.5 — Windows 10 x64, Go + SQLite (synthetic demo)

## What changed

- **Windows tray desktop** (`OrgAxis.exe`, Windows GUI subsystem): no CMD window; icon; quick launch HR Workspace and `/logs`; optional **Start with Windows** via HKCU Run; **Exit OrgAxis** stops local server.
- **Single instance**: Windows named mutex keyed by the SQLite data-directory path, shared across login sessions and service mode; separate session-local mutex for the tray UI. Linux uses `flock` for test/server mode.
- **Diagnostics**: `http://127.0.0.1:8765/logs`; local API `GET /v1/system/status`, `GET /v1/system/logs?lines=100`, authenticated with `X-API-Key`. Log rotation at 10 MiB × 4 backups. Do not share unreviewed logs.
- **Service**: `OrgAxis.exe -mode service` registers with Windows Service Control Manager when installed using `Install-OrgAxis-Service.ps1`. Runs as **LocalService**, headless, with automated service recovery. Service holds its **own** `%ProgramData%\OrgAxis\data` SQLite unless explicitly migrated. Tray client `OrgAxis.exe -mode client` attaches to service without opening another database.
- **Optional LAN**: explicit `-lan-host 192.168.x.x` on a server/service only; separate read-only HTTPS API at `https://LAN-IP:8766`, a randomly generated 256-bit bearer token in the service data directory, and a self-signed certificate. No HR UI, payroll, employee search or POST endpoints are exposed on LAN. The LAN listener is disabled by default.
- **Same Go + SQLite HR demo** with 1,000 synthetic staff, time-aware org and existing modules. No Docker, Python or Node needed on Windows to run either EXE.

## Run desktop (existing Windows 10 installation)

1. Stop your v0.4 tray process with **Exit OrgAxis** in the notification-area menu.
2. **Back up `data\orgaxis.sqlite3`** and retain the `data` directory.
3. Replace the old `OrgAxis.exe` with the new v0.5 `OrgAxis.exe` in the *same directory*. Run it by double-clicking; it binds to `127.0.0.1:8765`.
4. Tray → **View Logs / Diagnostics** or directly open `http://127.0.0.1:8765/logs`.
5. Tray → **Enable Start with Windows** creates an HKCU Run entry for the current user. Disable it in the same menu. It does not require admin rights.

## Windows Service (no active user login)

1. For an actual service deployment, close the desktop version or stop it first: both bind to port 8765.
2. Open **PowerShell as Administrator**, then run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\Install-OrgAxis-Service.ps1
Get-Service OrgAxis
```

The installer copies EXE into `%ProgramData%\OrgAxis\bin`, assigns LocalService write access only to `%ProgramData%\OrgAxis\data`, registers an auto-start Windows Service and starts it. The service creates an independent synthetic demo database by default; it does **not** migrate or modify your desktop `data` folder.

To attach the tray icon after logging in, run `Start-Tray-For-Service.cmd` (or `OrgAxis.exe -mode client`), and enable Start with Windows for that tray client if desired. The actual server continues running after logoff. The service is stopped/removed with `Uninstall-OrgAxis-Service.ps1` (data retained).

### Optional import of an existing demo database

Stop all processes first, make a SQLite-consistent backup, then install with:

```powershell
.\Install-OrgAxis-Service.ps1 -ImportExistingDemoData
```

This copies an already clean demo DB to service data **only if** no target DB, `-wal` or `-shm` file exists. It aborts rather than copying an active WAL database. Normal desktop upgrades preserve the existing SQLite file in place.

## LAN API (opt-in, synthetic data ONLY)

Start a headless server with `-lan-host` or install service with the LAN IP explicitly specified:

```powershell
.\Install-OrgAxis-Service.ps1 -LANHost 192.168.11.11
```

Port 8766 is the dedicated TLS listener and is firewalled to the local subnet for the service installer. The application itself must be bound to that LAN IP; it will not bind to `0.0.0.0` from this option.

After first start, on the **server machine** obtain:
- `%ProgramData%\OrgAxis\data\lan\cert.pem`: self-signed certificate (clients must explicitly trust or pin).
- `%ProgramData%\OrgAxis\data\lan\api-token.txt`: 64-character, randomly generated **secret** token; do not commit or share it publicly.

Example from a machine on the LAN (assuming a trusted copy of `cert.pem`):

```powershell
$token=(Get-Content .\api-token.txt -Raw).Trim()
curl.exe --cacert .\cert.pem -H "Authorization: Bearer $token" "https://192.168.11.11:8766/v1/people/EMP-0030/manager?as_of=2026-11-18"
```

Allowed read-only API routes: `org/blank`, `org/occupied`, `workflow/resolve`, `people/{id}/manager`, `people/{id}/chain`, `people/{id}/reports`. No writes, no payroll, no logs, no cross-tenant HR directory, no browser UI. Unauthenticated requests get HTTP 401; blocked routes get HTTP 404.

**This remains a synthetic-data demonstration, not an HR system approved for real employee information.** A single shared local demo API key, weak authorization, missing OIDC/RBAC, multi-tenancy and statutory payroll logic are known limitations. LAN is for demonstration of hierarchy/workflow only. Certificate and token are generated per LAN IP; changing the IP requires deliberate certificate regeneration and redistributing the new trusted certificate. Do not place the listener on the public internet.

## Test and build

```bash
go test ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w -H windowsgui' -o OrgAxis.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o OrgAxisServer.exe .
```

Windows 10 system tray and native SCM lifecycle have been **cross-compiled but cannot be executed in this Linux build environment**. Windows-side acceptance tests are still required for notification-area behavior, registry startup persistence, service stop/restart, LocalService permissions, and browser behavior.

## API paths

- Hero `http://127.0.0.1:8765/`
- HR Workspace `http://127.0.0.1:8765/app`
- Local Runtime Diagnostics `http://127.0.0.1:8765/logs`
- API docs `http://127.0.0.1:8765/docs`

Legacy sample data and editable module records stay in the original SQLite database if desktop is upgraded in place. SQLite database schemas and demo legal calculations are not production-ready.
