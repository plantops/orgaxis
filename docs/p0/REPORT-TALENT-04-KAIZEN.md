# OrgAxis P0 — Module TALENT-04: Sáng kiến & Thưởng Kaizen 360°

Assessment date 2026-11-19.
**OrgAxis integration maturity: 2.5/5. External Kaizen source maturity estimate: 3.0/5 (synthetic code-tested pilot); no live UAT or real payout reconciliation.**

## Source and architectural decision

Authoritative source code: `https://github.com/ngtrthanh/chinfon-kaizen`; target is Go+SQLite Windows standalone service. Hardened security dependency: Draft PR #1. P0 rollout changes: Draft PR #2 (`feature/p0-invite-without-phone-20261119`).

**One reward ledger only:** Kaizen is owner of initiatives, reward calculations, evidence, approvals and HR CSV. OrgAxis exposes a separate launcher configured by `ORGAXIS_KAIZEN_URL`. It does NOT duplicate Kaizen tables or copy sessions; Kaizen uses its own identity until an independently reviewed SSOT/SSO integration exists.

If the external link is configured, OrgAxis v0.6 BONUS creation returns HTTP 409 to avoid dual active idea/reward ledgers. Existing legacy records are retained for historical demo inspection.

## Verified current functions in Kaizen
- Three tracks: FAST48 (48h target), STANDARD (5 business-day screening, 30-day acceptance), CONFIDENTIAL (skip initial screening and restricted visibility); governance for track 3 requires explicit privacy authorization.
- Lifecycle DRAFT -> SCREEN -> REVIEW -> APPROVE -> IMPLEMENT -> APPLIED; reject, need-info and append-only workflow history.
- Financial reward tiers: T1 300k; T2 1m; T3 2.5m; T4 5m; T5 10m; T6 20m; T7 6% of net up to 100m VND. Annex 03 1.2x capped at 100m. Non-financial four tiers 1m/3m/8m/25m. Formula is in source, NOT legally/business certified.
- Submission by employee, co-author split, attachment SHA-256, private record access, review by role, final approval, HR payout export CSV and audit.
- Go single binary, embedded templates, SQLite, login, CSRF, MFA, import staff and no-demo mode.
- Source `main` still has predictable five-digit phone bootstrap PIN; security PR #1 replaces it with random expiring invitation and HTTPS binding rule. **Do not deploy `main` for real employees.**
- P0 PR #2 includes password-less mobile/contact import, strict graph preflight tool and a manual payment-reference gate/migration. Tests/build must pass and be reviewed before release.

Evidence: `internal/app/reward.go`, `workflow.go`, `staff.go`, `auth.go`, `handlers_review.go`, migrations, tests, `docs/PUBLIC_DEPLOYMENT_GATE.md`, `security/asvs/MIS_HANDOFF.md`.

## Data boundaries
Kaizen `md_employee` has `employee_no`, department and `report_to` (reference only). OrgAxis has `person` and temporal position reporting. P0 requires signed stable `employee_no` mapping, approved department and reviewer authority; do not grant role solely from a job title or `report_to`.

Initial rollout supports only explicitly authorized user/account cohorts. External app link is not SSO, is not a production network health check, and carries no identities. No automatic payroll data sync.

## Hard gates — no substitute by demo test
1. HR provides approved personnel and manager/role matrix; run strict preflight, stage in fresh non-demo DB, verify counts and updates.
2. MIS approves HTTPS TLS/secure-cookie endpoint with no public plain HTTP port; Tailscale ACL/firewall and platform access reviewed.
3. Security PR #1 and follow-on P0 PR #2 pass tests, dependency scan and independent security checks; test no-phone onboarding, MFA recovery, scope, Luồng 3 and attachments.
4. Finance/HR certify PN-B0000-02 reward rules, Annex 03, override exceptions, independent maker/checker and payout CSV.
5. Test with real authenticated staff on authorized workstations; at least one real, non-financially settled initiative with actual evidence; mark PAID only with externally verified payment document.
6. Backup DB+attachments+MFA key, execute restore drill; monitoring and owner/runbook named; agree retention/whistleblower handling.
7. MIS/HR/independent AppGate signoff and authorized DGD approval recorded.

## 2-day internal limited production rollout scope
Target dates (planning goal): Day 1 Nov 20; Day 2 Nov 21 2026. **Not a claimed deployment.** Do not mix with demo assets or announce 1,000-account launch. First cohort might be 10–30 real users with an approved HR list and an authentic idea/evidence; no real payout until Finance verified. Confidential whistleblower track stays restricted until privacy/process authorization.

Day 1: source freeze, CI, actual HR extract/review, OrgAxis manager graph signoff, Kaizen staging import, invite+MFA, HTTPS setup, backup/restore.
Day 2: HR/MIS UAT on distinct worker/supervisor/manager/council/HR roles; valid and denied endpoints; actual initiative path, evidence, reward review vs spreadsheet and payroll CSV. If any hard gate missing: internal pilot only, no authoritative release.

**CURRENT VERDICT: NOT GO-LIVE APPROVED.** Real HR data and hosting/production UAT are outside current source access; evidence cannot be fabricated. Source readiness and code tests do not substitute for company operations.
