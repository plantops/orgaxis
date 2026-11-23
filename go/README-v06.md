# OrgAxis v0.6 — Secure Workforce & Approval Demonstration

**Go + SQLite · Windows 10 x64 · single EXE · port 8765 · synthetic data only**

## Upgrade from v0.5

1. Stop the existing OrgAxis tray/server or Windows Service first. Backup the current `data/` folder.
2. Replace `OrgAxis.exe` with the v0.6 GUI EXE from this release in the **same** folder. Retain the existing `data/orgaxis.sqlite3`.
3. Double-click `OrgAxis.exe`, then open `http://127.0.0.1:8765/workspace` (or `/app`, which redirects to `/workspace`).
4. The first v0.6 start creates a consistent `orgaxis-before-v06-*.sqlite3` backup **before** authorization schema changes. The demo upgrades missing monthly payroll rows, and creates random role-specific user passwords at `data/DEMO-LOGIN-PRIVATE.txt`.
5. Do not send or publish the credentials file. Keep the entire pilot on a local demo machine. For Windows Service, the file is in `%ProgramData%\OrgAxis\data` and might require administrator access.

## Six local demo identities

| Username | Person | Role | Demo scenario |
|---|---|---|---|
| `employee` | EMP-0012 | EMPLOYEE | Create own leave request, own annual payslip |
| `manager` | EMP-0009 | SUPERVISOR | Approve direct reports, initiate reward/discipline cases |
| `hr` | EMP-0008 | HR | Review escalated cases (demo dual role) |
| `director` | EMP-0001 | DIRECTOR | Final approval, aggregate annual payroll |
| `payroll` | EMP-0007 | PAYROLL | 12-month financial ledger and individual payslips |
| `admin` | EMP-0002 | ADMIN | Account creation, policy, audit, gated legacy v1 API |

Each initial password is **randomly generated at first run**; there is no shared admin password embedded in source or HTML. The HR and payroll actor-role assignments in this demo are explicitly synthetic and do not imply actual positions in a real company.

## Org Chart

- Bird's-eye across the tenant: renders each of **1,000 fictional people** as an individual interactive mark inside their department.
- Department: managerial, shift-lead and operational personnel layers; per-person cards.
- Team/function: filter staff by position code.
- `as_of` permits time travel, using effective-dated assignments, reporting relationships and positions. Blank Org Chart remains independently dated.

## New scoped API

- `POST /auth/login`, `GET /auth/me`, `POST /auth/logout`: database-backed sessions.
- `GET /v2/org/full?as_of=2026-11-18`: all 1,000 staff in one graph payload (synthetic identities).
- `GET /v2/people/me`; `GET /v2/people/{person_id}`: per-person authorization.
- `GET /v2/payroll/year`: 12 aggregate months; PAYROLL / ADMIN / DIRECTOR only.
- `GET /v2/payroll/person/{person_id}`: own pay or PAYROLL / ADMIN.
- `GET /v2/cases?type=LEAVE|BONUS|DISCIPLINE`: own / initiated / assigned cases only.
- `POST /v2/cases`: leave submitted by logged-in employee; bonus/discipline initiated by eligible direct supervisor.
- `GET /v2/cases/{id}`: scoped case history.
- `POST /v2/cases/{id}/decision`: assigned reviewer only, server side; CSRF required.
- `POST /v2/cases/{id}/resubmit`: originator after NEED_INFO.
- `GET /v2/users`, `POST /v2/users`: ADMIN only; initial password returned once.
- `GET /v2/policies`, `POST /v2/policies`: configurable leave escalation thresholds (ADMIN writes).
- `GET /v2/audit`: ADMIN only.

Legacy `/v1` routes are **administrator-session-only** in the running v0.6. The old `X-API-Key` demo header no longer bypasses browser authorization. Separate opt-in TLS read-only LAN bearer routes from v0.5 are retained solely for synthetic organization/hierarchy data.

## Approval examples

- Up to **2 synthetic workdays**: employee → line manager.
- 3–5 workdays: employee → line manager → HR.
- Above 5 workdays: employee → line manager → HR → director.
- Two thresholds can be changed in the admin security screen; the current request's approval route is a snapshot, unchanged if the policy or organizational chart subsequently changes.
- Demonstration calendar uses Monday–Saturday and does **not** include public holidays, shift patterns, employment contracts, leave entitlement or statutory calculations.
- Reward: direct manager starts → HR review → director final; financial T1–T7 and non-financial NF1–NF4 amounts modeled after the `ngtrthanh/chinfon-kaizen` source implementation for **demonstration**, with financial/Annex03 inputs.
- Discipline: direct manager initiates factual case → HR review → director review. A completed workflow does **not** automatically impose a labor-law sanction.
- No self-approval; no unassigned approvers; no out-of-scope review; all transitions audited with an actor ID.

## 2026 payroll

12 periods × 1,000 people = **12,000 synthetic payslips**, with base pay, allowances, overtime, attendance and performance bonuses, illustrative insurance, estimated PIT and net. Aggregate annual ledger and scoped personal payslips are available. **These values and tax bands are not verified statutory pay or tax computations.**

## Security limitations

This is a deliberately local synthetic demo. It includes PBKDF2-HMAC-SHA256 password hashing (100k rounds), random session cookies (HttpOnly, SameSite Strict), 8-hour expiry, CSRF tokens, simple per-IP+username login throttling, backend RBAC + subject/manager scope and audit of writes.

It does **not** include OIDC/SSO, TOTP MFA, password reset, employee provisioning from authoritative systems, production secret management, hardened key rotation, tenant isolation, fine-grained medical fields, or independent security assessment. Keep private employee data out of the demo.

## Validate

```bash
go test ./...
node --check assets/workspace.js
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w -H windowsgui' -o OrgAxis.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o OrgAxisServer.exe .
```

Windows 10 native tray/SCM behavior remains to be checked on the target machine. Preserve the database and backup before replacing binaries. The service installer/uninstaller scripts are inherited from v0.5.
