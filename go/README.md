# OrgAxis Workforce Workspace — Go + SQLite (Win10 x64)

OrgAxis is an **API-first temporal organization/workforce engine pilot** with an interactive HR workspace. The pilot database and all personnel are **100% synthetic**.

## Start on Windows 10 (without Docker)

1. Stop the old process using `Ctrl+C`.
2. **Keep the existing `data/` directory**. Back it up if it contains demo edits you need.
3. Extract the release and copy `OrgAxis-Windows10-x64.exe`, `Start-OrgAxis.cmd` into your existing OrgAxis directory, overwriting the old executable.
4. Run `Start-OrgAxis.cmd` or `OrgAxis-Windows10-x64.exe`.
5. Open `http://127.0.0.1:8765/` (showcase), `http://127.0.0.1:8765/app` (workspace), or `http://127.0.0.1:8765/docs` (API reference).

The executable uses native Windows `winsqlite3.dll` and embeds the HTML/CSS/JS assets plus a seeded SQLite template. It requires **no Docker, Python, Node, or additional installer**.

The engine listens on `127.0.0.1:8765` by default. The demo API key is `demo-local-key`. Do not expose this binary outside the local host; it is not a secure multi-tenant deployment.

### Existing database upgrade

If `data/orgaxis.sqlite3` exists from the previous pilot, OrgAxis retains that file. On first upgrade when the skill catalog is empty, the engine:

1. Creates a consistent SQLite backup `data/orgaxis-before-module-upgrade-YYYYMMDD-HHMMSS.sqlite3` (or aborts if a backup cannot be created).
2. Adds the missing workflow table and supporting indexes.
3. Adds deterministic synthetic records to empty module tables, using `INSERT OR IGNORE` with stable fixture IDs.
4. Replaces only names matching `Synthetic Employee %` with synthetic display names; records with names edited by the user remain unchanged.

Do not delete the `data/` directory. Upgrades never overwrite it with the embedded seed template. A fresh installation uses the fully seeded database.

Custom launch:

```powershell
.\OrgAxis-Windows10-x64.exe -port 8765 -no-browser
```

## Scope implemented

| Domain | Read | Create / change | Scope / limitation |
|---|---|---|---|
| Blank & occupied temporal organization | Yes | Dated reporting change | Changes to unit structure require a future change-set workflow |
| Person & employment | Yes | Demo hire, move, separation | Synthetic only; no legally compliant contract lifecycle |
| Workforce skills | Yes | New assessments | Skills L1–L4 are internal; no license is inferred |
| Dispatch skill eligibility | Yes | Skill-only check | Always returns `authorized_to_work: false`; does not assess permits/training/fitness |
| Training & qualifications | Yes | Training record | Qualification/certification issuance not automated |
| Performance & KPI | Yes | KPI result | Not a payroll or disciplinary decision |
| Leave & absence | Yes | Create, approve, reject | Approval lacks real user RBAC |
| Workflow & approvals | Yes | Create, approve, reject | LINE_MANAGER routing only; no delegation or spending limits |
| Recognition & disciplinary cases | Yes | Proposal / open case | No legal disciplinary decision engine |
| Occupational exposure/history | Yes | Read-only | Classifications are synthetic, **not** legal entitlement determinations |
| Payroll, tax, social insurance | Yes | Benefit claim create | **Record-only**, no statutory calculations, no real payslips |
| Immigration & foreign-worker compliance | Yes | Read-only | Masked synthetic passports/permits, not legal verification |

### Synthetic seed — Cement Factory Pilot

- `tenant-cfc-pilot`, one synthetic company
- 1,000 fictitious employees in two plants
- 2 blank organization structural changes and 80 workforce transfers
- 16 skills, 3,636 skill assessments
- 12 training programs, 4,641 training records
- 9,984 KPI result rows
- 175 absence cases, 90 recognition cases, 12 disciplinary cases
- 1,058 occupational periods, 32 workflow requests
- 1,000 synthetic compensation agreements, 1,000 tax profile rows, 1,000 social insurance period rows, 50 benefit claims
- 18 simulated foreign experts and masked immigration documents

The original records are retained in `data/orgaxis.sqlite3`; the downloadable binary will not connect to a real ERP.

## API endpoints

The full reference is available at `/docs`. All `/v1/` endpoints require `X-API-Key`. Query strings use ISO calendar dates (`YYYY-MM-DD`) and monthly KPI periods (`YYYY-MM`).

```powershell
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:8765/v1/dashboard?as_of=2026-11-18"
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:8765/v1/people/search?q=EMP-0030"
curl.exe -H "X-API-Key: demo-local-key" "http://127.0.0.1:8765/v1/workflow/resolve?subject_id=EMP-0030&as_of=2026-11-18"
```

## Development / build

Requires Go 1.23+, and on Linux, CGO with libsqlite3 for testing. Windows uses the system SQLite DLL and cross-compiles without CGO.

```bash
# Start from the repository root (root seed.py is part of the bootstrap branch):
cd go
ORGAXIS_TENANT_ID=tenant-cfc-pilot ORGAXIS_SEED_PROFILE=single_company ORGAXIS_TENANT_CODE=CFCPILOT ORGAXIS_TENANT_NAME='Cement Factory Pilot Tenant' ORGAXIS_DEMO_COMPANY_CODE=CFCPILOT ORGAXIS_DEMO_COMPANY_NAME='Cement Factory Pilot (SYNTHETIC)' python3 ../seed.py --db assets/seed.sqlite3
python3 enrich_seed.py
python3 enrich_extended.py
python3 make_demo_assets.py
go test ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o OrgAxis-Windows10-x64.exe .
```

## Security and commercial readiness gaps

This is a **functional local pilot**, not production SaaS. Identity/authentication, session management, RBAC, tenant isolation, field-level HR privacy, legal rules, consent, payroll and entitlement calculations, integration adapters, structured migrations, import/export validation and release signing are not complete. Current demo actions accept a shared local API key; approval events do not prove the named manager approved them. Treat health, discipline, salary and identification as highly sensitive data; do not import real data into this build.

The browser showcase and app UI are hand-built and tested for asset serving and JavaScript syntax. No complete native Windows 10 browser acceptance test has been performed in this environment.
