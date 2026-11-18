# OrgAxis — Windows 10 x64 / Go + SQLite

Standalone demonstration of **Temporal Organization & Workforce** with a sales-focused landing page for HR managers.

## Windows 10 / no Docker, no Python

1. Extract `OrgAxis-Win10-GoSQLite.zip` to a writable folder, e.g. `C:\OrgAxis`.
2. Double-click `OrgAxis-Windows10-x64.exe` (or `Start-OrgAxis.cmd`).
3. Open `http://127.0.0.1:6666/` in a browser.
4. Click `Mở demo` for `http://127.0.0.1:6666/app` and enter API key `demo-local-key`.

The `.exe` includes all HTML assets and a *synthetic* database template. At the first run, the executable creates `data\orgaxis.sqlite3` beside itself. Existing databases are preserved on subsequent runs. **Do not delete the `data` directory if you need to keep edited demo data.**

- Hero: `/`
- App: `/app`
- API reference: `/docs`
- API health: `/health`
- Port: `6666` (default)
- Listen address: `127.0.0.1` only
- Backend: Go standard library + Windows' `winsqlite3.dll`
- Storage: SQLite database in local `data` directory
- Additional dependencies: no Docker, Python, Node, or separate SQLite install.

Command-line options:

```powershell
.\OrgAxis-Windows10-x64.exe -port 6666 -no-browser
.\OrgAxis-Windows10-x64.exe -data "D:\OrgAxisData" -api-key "change-this-demo-key"
```

The demo creates one tenant, one synthetic company, 1,000 fabricated employee records, two dated blank-org structural changes and 80 fictional transfers. All records are entirely synthetic.

## API examples

```powershell
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:6666/v1/tenant"
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:6666/v1/org/blank?as_of=2027-01-01"
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:6666/v1/people/EMP-0030/manager?as_of=2026-11-18"
```

## Go development

The GitHub Actions build generates `go/assets/seed.sqlite3` from the existing
root-level `seed.py` before compiling the self-contained executable. For local
builds from a source checkout, create the fixture once:

```sh
ORGAXIS_SEED_PROFILE=single_company \
ORGAXIS_TENANT_CODE=CFCPILOT \
ORGAXIS_TENANT_NAME="Cement Factory Pilot Tenant" \
ORGAXIS_DEMO_COMPANY_CODE=CFCPILOT \
ORGAXIS_DEMO_COMPANY_NAME="Cement Factory Pilot (SYNTHETIC)" \
python ../seed.py --db assets/seed.sqlite3

go test ./...             # Linux with CGO + libsqlite3
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o OrgAxis-Windows10-x64.exe .
```

The Windows build imports the SQLite implementation from `winsqlite3.dll` bundled with supported Windows 10 versions. The project has no third-party Go dependencies. The Windows binary has been cross-compiled on Linux; **native Win10 execution has not been verified** in the build environment.

## Security boundary

This is **not a production SaaS/HRIS**. The API uses a shared demo key with no tenant-bound OIDC or RBAC. It listens only on local loopback. Do not expose it to other computers, the public Internet, or load real employee, health, payroll, tax, or disciplinary data into the demo.

Go API parity is scoped to existing pilot functionality. No payroll, statutory benefits, immigration, advanced authorization policy, or sensitive personal-data handling is implemented.

## Package structure

- `main.go`: embedded server, tenant API, reporting, workflow, temporal changes
- `sqlite_windows.go`: native Windows SQLite binding, no CGO
- `sqlite_linux.go`: local Linux validation via system SQLite
- `assets/hero.html`: marketing landing page
- `assets/index.html`: tenant demo console
- `assets/docs.html`: API reference
- `assets/seed.sqlite3`: fictional data used for first-run initialization
- `main_test.go`: HTTP-level tests against seeded database
