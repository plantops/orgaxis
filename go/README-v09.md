# OrgAxis P0 v0.9 — monthly time ledger review and lock

**Status: engineering pilot / draft.** This does not implement Vietnamese official payroll posting, wages, tax or social insurance. No UAT signoff.

## Time ledger contracts (authenticated `/v2`, CSRF for mutations)

1. `POST /v2/time/calendar/import` — HR/ADMIN. JSON keys: `person_id`, `period` (`YYYY-MM`), `source_ref` (external evidence identifier), `days` (every calendar date of the month, each object with `date` and `kind` = `SHIFT`, `REST`, `HOLIDAY`). Cannot edit a locked period.
2. `GET /v2/time/period/readiness?person_id=...&period=...` — HR/PAYROLL/ADMIN. Evaluates each scheduled shift using accepted immutable raw events, approved time credits and day plans; returns explicit blockers and a deterministic SHA-256 digest of source records.
3. `POST /v2/time/period/prepare` — HR/ADMIN, body `{"person_id":"EMP-0012","period":"2026-02"}`. Returns 409 on blockers; otherwise records a PREPARED time ledger snapshot.
4. `POST /v2/time/period/approve` — PAYROLL only, same body. Distinct actor from preparer, re-evaluates readiness and source hash, then transitions PREPARED to LOCKED.
5. `GET /v2/time/period/verify?person_id=...&period=...` — checks whether current source rows still match the locked/prepared snapshot; a later raw event gives `source_unchanged=false`.

A rejected request has no partial month lock. Approved `oa_time_month_close` rows, day plans and rosters have SQLite mutation guards. Raw punches continue to be append-only: late source events remain available for audit and are **not silently folded into a locked snapshot**.

## Safety and scope

- A month requires 28–31 explicit plan dates. Missing date plans, missing roster, anomalous IN/OUT, unrostered punches, quarantine and synthetic sources block preparation.
- Until actual approved case evidence is mapped to `oa_time_leave_credit` and `oa_time_ot`, any leave/OT credits block time ledger lock. Approved status flags in demo tables alone are not enough.
- `source_ref` records an evidence identifier and is not independent verification. Test fixtures can demonstrate mechanics, not HR UAT acceptance.
- SHA-256 helps detect source drift but **is not WORM or independent notarization**; a privileged DB administrator can still alter/destroy SQLite or bypass triggers.
- Off-day and holiday shifts require additional real plant scheduling policy and overtime treatment before UAT.
- Locks are per person/month, not batch payroll posting; corrections, reopening, retro-pay, bank export, BHXH and PIT are not implemented.

## Upgrade/rollback

Stop Windows service and tray. Back up the entire `data` directory before replacing executables. Keep `data/orgaxis.sqlite3` untouched. On first run after v0.8, the application saves a consistent `orgaxis-before-time-ledger-*.sqlite3` snapshot using `VACUUM INTO` before additive tables and guards are installed. Test restore on a copy before actual employee records are used.

## Evidence

Run `go test ./...`, `go test -race ./...`, `go vet ./...`. `attendance_close_v09_test.go` verifies a full 28-day synthetic fixture, two distinct approvers, roster/ledger immutability, rejection of incomplete synthetic periods, RBAC/CSRF and detection of late raw events. No physical device, native Windows 10, restoration of a real DB, or independent AppGate assessment was performed by these tests.

Stacked PR order: #6 -> #7 -> #9 -> #10. Do not merge to main out of order.
