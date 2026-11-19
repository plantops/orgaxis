# OrgAxis — P0 three-module rollout decision (2026-11-19)

**Only in-scope modules for this rollout: ORG-03, PERSON-01, TALENT-04 Kaizen 360.**
All other OrgAxis modules are deferred. PERSON-05 Recruitment/ATS has been explicitly added to catalog as `not_implemented`, 0/5 (backlog), and will not be built in this sprint.

## Status matrix — engineering readiness vs actual live acceptance

| Module | Source maturity /5 | Current test data | Live data/UAT | Current decision | Separate report |
|---|---:|---|---|---|---|
| ORG-03 Position & Reporting | 3.0 | 1,000 synthetic / shared position templates | Missing; `person.synthetic=1` schema | BLOCKED | `REPORT-ORG-03-POSITION-REPORTING.md` |
| PERSON-01 Employee Dossier | 3.0 | Synthetic person/employment/assignment | Missing HR extract & mapping | BLOCKED | `REPORT-PERSON-01-EMPLOYEE-DOSSIER.md` |
| TALENT-04 Innovation Reward / Kaizen | 2.5 (OrgAxis module integration); standalone Kaizen source estimate 3.0 | Kaizen code/tests; embedded demo master hidden in non-demo | P0 code PRs, no signed real UAT | BLOCKED | `REPORT-TALENT-04-KAIZEN.md` |

**No module is 4/5 or 5/5. No AppGate / OWASP ASVS certification or independent company UAT.**
Maturity is source assessment, not production operations.

## Architectural decision

Run `ngtrthanh/chinfon-kaizen` as its own Go+SQLite application/DB and single **authoritative initiative/reward ledger**. Link via authenticated OrgAxis Workspace navigation using `ORGAXIS_KAIZEN_URL`; no raw cross-application credential, session or personal data forwarding. Until approved SSO and HR SSOT mapping exists, Kaizen must manage its own users with security PR #1 + P0 PR #2.

**Danger of parallel ledgers:** OrgAxis has an older v0.6 BONUS demo case/workflow; when Kaizen endpoint is configured, creation of new BONUS records is explicitly blocked with HTTP 409. Preserve old data for audit only; do not migrate mock BONUS into real Kaizen. Avoid cross-repo code duplication.

Dependency graph:
```
HR-approved actual employee number + position/reporting effective dates
        |                          |
        v                          v
PERSON-01 (real SSOT)        ORG-03 (real reporting/authority)
        \                          /
         \---- approved scoped reviewers ---/
                       |
                       v
TALENT-04 = Chinfon Kaizen 360 independent service and reward ledger
                       |
                       v
HR + Finance reconciled payout document -> audited PAID
```
**Critical uncovered dependency:** OrgAxis presently cannot ingest or serve real persons under its synthetic-only person schema. No real HR roster or hosting control is available to this development session. The new data validation tool is useful preflight but not evidence that ORG-03/PERSON-01 source-of-truth exists. This is a **hard no-go** if the three OrgAxis modules are all required live before Kaizen.

## Concrete current code changes

**OrgAxis P0 branch** `feature/p0-org-person-kaizen-live-v12`:
- `go/kaizen_module_v12.go`: validated HTTPS-only (or local loopback) Kaizen URL from env; authenticated `/v2/kaizen/launch`; separate-login/no-PII warning.
- `go/workflow_v06.go`: block legacy BONUS POST when external Kaizen configured.
- `go/assets/workspace.html`/`.js`: standalone Kaizen view, visible dependency and scope.
- `go/module_catalog_v11.go`: TALENT-04 now references external Kaizen source, PERSON-05 Recruitment/ATS = 0/5 backlog.
- `go/kaizen_module_v12_test.go`: URL/auth/legacy ledger fence regression (CI result must be verified).

**Kaizen hardened P0 branch** `feature/p0-invite-without-phone-20261119`, stacked on Draft security PR #1:
- Import all employees as PENDING even without personal phone or email; random 30-minute `issue-invite`, forced new password and TOTP.
- Council user creation without mobile.
- `tools/p0_hr_preflight.py` + tests: strict HR input validation; duplicate IDs, missing manager, reporting cycle, malicious spreadsheets and CSV privilege escalation hard-fail; optional clean CSV to output.
- At IMPLEMENT -> APPLIED, require an HR-checked external payment reference (migration 010) instead of a single approval click silently producing PAID.
- CI workflow tests P0 HR preflight. New code and P0 payout workflow require full CI revalidation and independent HR/MIS UAT.

## Two-day execution calendar (target, not claimed delivery)

**Friday 2026-11-20 — Day 1: Make ORG-03 and PERSON-01 source trustworthy.**
- HR provides a signed minimal staff/organization export with code, department, job/slot, effective manager and active status; no DOB/private phone/salary needed.
- Check root / Board of Members / Supervisory Board / CEO / deputy / manager / supervisor / foreman / workers in the actual approved structure, separating governance from LINE.
- Run the P0 graph preflight and reconcile to HR as of the approved effective date. Check cross-unit managers, cycles and identity uniqueness; HR approves resulting roster hash.
- Separate real DB from embedded OrgAxis synthetic demo. Until migration/import supports non-synthetic persons and positions, mark ORG-03/PERSON-01 BLOCKED, not PASS.
- Finalize source branches/CI, build production Kaizen isolated instance, configure HTTPS, backup + restore and two administrator break-glass identities. Do not open confidential track publicly.

**Saturday 2026-11-21 — Day 2: Limited real-user UAT and go/no-go.**
- Use small real authorized cohort (e.g. 10–30 users) only if HR signs personnel/reviewer scope; test worker/no-email, foreman, manager, council, HRD/Finance.
- Issue individual random invites, test MFA and recovery; security tests on IDOR, cross-department, Luồng 3 confidentiality, CSRF, upload, login, and role changes.
- Walk one authentic idea (not demo) from DRAFT through screening/review/approval with evidence; validate reward vs approved PN-B0000-02; **do not mark payment until external bank/HR document exists and is verified**.
- MIS tests TLS/cookies, backup restore, monitoring, audit and incident/rollback. Finance validates CSV and maker-checker.
- At final checkpoint: if ORG-03/PERSON-01 real master remains blocked, or security/payment/privacy hard gates are open: **NO-GO for authoritative production**; only isolated pilot/limited demonstration on authorized real data after privacy approval.

## Recruiting answer

There was **no proper Recruitment/ATS module** in the 35-module v0.11 inventory; `PERSON-02 Hire/Transfer/Separation` only covers employment events after a staffing decision, not requisition, candidate applications or interview pipeline. New `PERSON-05 Recruitment & ATS` is tracked at 0/5 and explicitly deferred. Future workflow: approved Position Slot -> recruitment request -> candidates -> structured interview -> offer -> onboarding -> hire/assignment. Candidate records need separate privacy boundary from existing employee master.

## Required external artifacts and accountable sign-off

1. HR: employee/master/position/reporting export + official effective date and row reconciliation.
2. Business sponsor: signed PN-B0000-02 tier/approval policy and confidentiality workflow.
3. MIS: approved DNS/HTTPS endpoint, ACL, backup encryption, restore drill, admin runbook, P0 source review.
4. Finance/HRD: payment evidence and CSV comparison, named maker/checker and initial case acceptance.
5. Independent security/AppGate reviewer: evidence, open findings, signed verdict.

**This document is not evidence of actual company deployment.**
