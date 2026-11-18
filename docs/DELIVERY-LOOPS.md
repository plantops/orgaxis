# OrgAxis delivery loops — evidence based

## Loop A — Baseline and competitor benchmark (COMPLETED in current turn)
- Read OrgAxis v0.6, Kaizen and AppGate GitHub sources.
- Compare MISA AMIS, Base HRM+, 1Office, Tanca against SAP SuccessFactors, Workday, Oracle Fusion and UKG.
- Core problem: no controlled `roster -> raw -> approved changes -> calculated time -> payroll` chain.
- Output: `docs/HRM-BENCHMARK-2026.md`.

## Loop B — P0 attendance slice (PROTOTYPE IMPLEMENTED AND LOCALLY TESTED)
- New `timecore`: deterministic pairing, strict IN/OUT, duplicate source key, midnight crossing, approved paid leave and OT, exception-based block of close.
- New `oa_*` tables: raw time punches, dated employee badge binding, roster, device catalog, approved leave/OT credits, quarantine.
- Scoped APIs: `GET /v2/time/day`, `/quality`, `/pay-preview`, `/devices`, `POST /v2/time/punches/import`; RBAC+CSRF.
- Sample seed: one fake terminal; 1,000 badge mappings, 6,000 roster shifts and 11k+ punches for Nov 2–7, 2026; edge cases missing OUT, full-day leave, approved OT, unapproved OT.
- Read-only vendor-neutral JSON API adapter with cursor; **no physical vendor integration**.
- UI `/time` preview, high-contrast desktop, accessible nav from `/workspace`.
- Outputs: `attendance_v07*.go`, `timecore/*.go`, assets/time.*, `docs/ATTENDANCE-API.md`.

## Loop C — Quality and AppGate baseline (PARTIAL, not production PASS)
- Tested code with `go test`, targeted replay/permission tests, `go vet` and Windows cross-compile where available.
- AppGate mapping and unsatisfied gates documented in `docs/APPGATE-HR-READINESS.md`.
- Missing: tests of a physical clock, manufacturer API, restored DB, employee calendar, official payroll rules, native Windows UI acceptance.

## Loop D — Next P0 release, 2 weeks of implementation scope, not yet delivered
1. **Time device adapter**: select the actual model/protocol, source badge-ID mapping, cursor/checkpoint, pull interval, drift alarms and source SHA256/audit; edge spool when LAN unavailable.
2. **Calendar/roster**: shifts A/B/C, overnight, personal roster, effective-dated work pattern, meal breaks, holidays, weekend/plant shutdown, replacement shifts and real overtime consent.
3. **Time ledger**: daily exception queue, supervisor review, leave/OT approval binding, bulk reconciliation, monthly lock with snapshot/hash, re-open requiring independent approval.
4. **Wage components**: hourly salary basis, paid leave, unpaid leave, attendance/shift/night/OT premiums, payroll period, rounding, union/company agreement, Vietnam legal packs effective dated.
5. **Reconciliation**: 100 golden employees for 2 months, compare existing HR payroll; differences 0 on fixed cases, documented tolerances for approved rounding; exceptions resolved by HR/payroll.

## Loop E — AppGate UAT and external pilot (BLOCKED until D complete)
- 49-criterion AppGate assessment, evidence for hard gates; target >=80/100 and 100% coverage per existing rubric, independent reviewer.
- Role-based UAT: employee, supervisor, HR, payroll, director, admin; 24h plant operation scenario.
- Security: MFA/SSO, service accounts, tenant/employee authorization, salary privacy, audit, encryption and backup/restore.
- Performance: >=1,000 people, >=20,000 punches/day burst, 12 months, p95 and outage replay target agreed in UAT.
- **No payroll production release** until approved wage calculations and official bank/disbursement procedures.

## Time-effective master-data invariants
- The blank org chart, reporting edges, assignments, shift rosters, badge bindings, payroll contracts, attendance rules and approvals have effective dates.
- Device punch is fact evidence, not assignment/authority. No employee can approve own leave or own OT.
- Calculated day references the source punches + roster version + leave/OT decision version + engine rule version.
- A retroactive change never silently rewrites a locked payroll month; create an adjustment in a later run, with before/after and reason.
- Multi-employer support: same person may have parallel employment episodes; payroll employer and cost center must be explicit per earning.
