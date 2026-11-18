# OrgAxis v0.7 — Time & Attendance Vertical Slice

Go + SQLite · Windows 10 x64 · no Docker · 127.0.0.1:8765 · **synthetic demonstration only**

## What is new

- `/time` time-and-attendance quality/workday/pay-preview UI. Log in to `/workspace` first.
- Normalized event import through authenticated, CSRF-protected `POST /v2/time/punches/import` (HR/Admin only); stable source/device/event key; quarantine unknown badges; strictly append raw accepted events.
- 1,000 badge mappings, 6,000 roster shifts (2–7 Nov 2026) and 11,977 synthetic punch events from one simulated terminal; scheduled overnight shift; synthetic leave/OT cases and deliberate missing punch exceptions.
- Pure Go `timecore` independent of HTTP and SQLite: punch pairing, duplicates, shift overlap, approved paid leave and overtime, exception blocking and gross-only preview.
- `timecore.HTTPPullReader` reads a vendor-neutral JSON API via HTTP GET and cursor, TLS outside loopback; no device manufacturer protocol implemented yet.
- Scoped APIs and synthetic data preserve v0.6 Person, Organization, Reporting, RBAC, Leave and 12,000 payslip demo rows.

## Upgrade from existing v0.6

1. Stop Tray and Windows Service before copying EXE. Make a manual backup of the entire current `data` directory.
2. Replace **only** `OrgAxis.exe` and/or `OrgAxisServer.exe` in the existing directory. Do not delete `data/orgaxis.sqlite3`.
3. Start `OrgAxis.exe`. It performs a consistent SQLite `VACUUM INTO` backup with prefix `orgaxis-before-v07-` before adding time tables, then loads the synthetic fixture when no roster exists.
4. Open `http://127.0.0.1:8765/workspace`, sign in with existing demo account, follow link **Công · Ca · Lương** to `http://127.0.0.1:8765/time`.
5. Do not change this pilot into a production HR deployment; confidential staff and actual device credentials are not supported safely here.

## Demo role

Use existing `data/DEMO-LOGIN-PRIVATE.txt` credentials from the previous installation. For a clean pilot, a new randomized credentials file is generated. `employee` sees own timecard; `hr` or `admin` sees importer quality; `payroll` sees scoped pay preview, which stays **BLOCKED** if the period isn't complete.

## Validation

```sh
go test ./...
go test -race ./...
go vet ./...
```

Linux synthetic server starts with `-mode server -port 8765 -no-browser`. The Windows binaries are cross-compiled and require native Win10 acceptance for tray/SCM. The separate `smoke-v07.py` script exercises auth, RBAC, import replay and payroll-block behavior against a synthetic local instance.

## Product benchmark and gate

- [Competitive benchmark](docs/HRM-BENCHMARK-2026.md)
- [AppGate design-stage intake](docs/APPGATE-HR-READINESS.md)
- [Iteration plan](docs/DELIVERY-LOOPS.md)
- [Attendance API contract](docs/ATTENDANCE-API.md)

The underlying AppGate rubric is owned by `plantops/app-gate`; this branch does **not** modify the rubric or claim to have achieved its 80/100 UAT acceptance requirement.

## Explicit blockers

1. No physical time machine/vendor integration, no persistent adapter cursor or unattended polling service principal.
2. No complete VN holidays, breaks/shift calendars, OT legal compliance, actual payroll computation, BHXH/PIT or payroll close/re-open.
3. Raw events have an immutable insert API but no WORM archive, production migration ledger, off-machine secure backup or independent penetration test.
4. No actual employee data, contract import, authorization with OIDC/MFA or independent AppGate UAT.
