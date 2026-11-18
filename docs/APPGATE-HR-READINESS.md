# OrgAxis × PlantOps AppGate — assessment intake & P0 gates

- **Assessment candidate:** OrgAxis v0.7 `feature/time-attendance-core-v07` (Go + SQLite pilot)
- **Source rubric:** `plantops/app-gate/framework/enterprise-v2026.11.2.yaml` (**49 criteria; ten weighted categories**), not a newly invented scoring framework.
- **Rubric categories/weights:** BIZ 17; DAT 14; SEC 13; INT 12; REL 12; DEL 10; OPS 8; UX 5; GOV 5; COM 4.
- **Stage:** `design` initially; `uat` only after real-machine integration and HR verified examples.
- **Mode:** `build`.
- **Criticality:** `high` (personal records and payroll). Escalate IT/OT-dependent deployment review when connecting to device networks.
- **Status:** INTAKE ONLY. **No AppGate assessment was created in the running AppGate instance, no scores were awarded and no gates were signed off.**

## AppGate acceptance policy (source-defined)

At UAT/go-live: score >=80/100, coverage=100%, all hard gates pass with evidence, independent approver distinct from submitter. `design` is not go-live approval. Preserve an immutable framework version/hash snapshot per assessment. See `plantops/app-gate/docs/ARCHITECTURE.md`, `docs/RUBRIC_POLICY.md`, `docs/PRODUCTION_GATES.md`.

## Blocking evidence map (do not mark PASS from source code alone)

| Criterion | Current P0 evidence | Missing before AppGate UAT |
|---|---|---|
| BIZ-02 Business invariants | `timecore/engine_test.go`: duplicate/missing/OT/leave | Plant shift calendar and HR verified reference timesheets |
| BIZ-03 Workflow | Existing v0.6 `workflow_v06.go` | Attendance correction / escalation / overtime consent / pay approval |
| DAT-01 Master data | Person, position and time-effective `oa_badge_binding` | Authoritative ERP person mapping and device identity owner |
| DAT-02 Transactions | SQLite `BEGIN IMMEDIATE` batch; duplicate event key | Concurrent replay, crash/retry, simulated database failure |
| DAT-03 Change history | Raw event retention, audition of importer | Immutable correction versions, as-known-at ledger |
| DAT-04 Accuracy | Minute unit and midnight tests | Golden set: 100 workers x 2 payroll months, legal-approved rounding |
| DAT-05 Migration | Additive pilot tables | Restore rehearsal from v0.6, WAL snapshot and rollback |
| DAT-06 Provenance | Source/device/event ID/timestamp, quarantine | Device clock skew, missing/late batches, source integrity |
| SEC-01 Identity | v0.6 local accounts | Production OIDC/MFA, device service account lifecycle |
| SEC-02 Server authorization | Time API HR/ADMIN and scoped payroll | Tenants/legal employers, salary confidentiality attack tests |
| SEC-03/04 Security/privacy | No data from real staff; source restrictions | ASVS-based assessment, encryption, data inventory, field-level controls |
| INT-01 API contract | `/v2/time/*` versioned API in source | OpenAPI, golden external consumer test |
| INT-02 Idempotence | Replay and conflicting event tests | Durable cursor/checkpoint, retries with outage |
| INT-04 OT gateway | `timecore/adapter.go` HTTP JSON reader mock | Vendor model API, OT firewall/gateway and failover |
| REL-01 Backup | Existing SQLite backup features | Restore RPO/RTO drill with punches and payouts |
| REL-04 Capacity | 1k synthetic people, 6k shifts, 11k+ punches | Peak test with 1k workers x 12 months; p95 + DB growth |
| DEL-02 Regression | Go unit and integration tests | Windows 10 runner and native device simulator CI |
| GOV-02 Audit | Imported batch audit in `oa_audit` | Approval identity, immutable payroll closing ledger |
| GOV-04 Privacy | Synthetic-only pilot | Vietnam data privacy inventory, purpose limitation, retention |

## Assessor responsibilities and hard rules

HR Owner validates payroll calendars, entitlements and exception dispositions; Finance/Payroll Owner validates pay components, rounding and reconciliation; OT/IT Owner validates the physical device/gateway; Security Owner validates SSO, scopes and device API access. An AppGate independent reviewer must sign off separately from the developer and evaluator.

**Release block:** Even a positive synthetic test cannot authorize real salaries or production access. Payroll must have close/lock, reversal, retrospective correction, bank export reconciliation, independent approval and secure storage.
