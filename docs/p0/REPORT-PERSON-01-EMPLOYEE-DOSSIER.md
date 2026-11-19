# OrgAxis P0 — Module PERSON-01: Employee Master & Dossier

Assessment date 2026-11-19.
**Engineering maturity: 3.0/5 (synthetic test pilot). Live master data quality: NOT VERIFIED.**

## Functions present in OrgAxis source
- Temporal `person`, `employment_episode`, `assignment`, HR events, qualifications and dossier.
- Authenticated `GET /v2/people/me`; role- and manager-scope-gated `GET /v2/people/{id}`.
- Staff attributes including employment/occupation history, skills, training and restricted record views.
- Synthetic records for 1,000 people and tests of role-based person read and date-dependent reporting.

Evidence: `schema.sql`, `go/modules.go`, `go/workflow_v06.go`, `go/security_v06.go`, `go/security_v06_test.go`.

## Production blockers
- Demo person IDs `EMP-xxxx` are synthetic. Table `person` has `synthetic INTEGER NOT NULL DEFAULT 1 CHECK(synthetic=1)`. This explicitly disallows a real profile while retaining current schema.
- No verified HR/ERP SSOT import and no employee-ID reconciliation against `chinfon-kaizen.md_employee.employee_no`.
- Name, department, position title, status, managerial position and account identity are currently different shapes across the two repos. Copying the demo database or granting broad read access is not acceptable.
- Sensitive employment history is not required for Kaizen P0. Use a minimal verified subset and strict need-to-know access.

## Minimum HR input
Required for Kaizen case workflow: stable employee_no, department_code, current active flag, approved display name, position slot ID, manager approval scope and effective dates. Optional email. **Personal phone, DOB, salary, tax, insurance and private identity documents are NOT necessary** for initial initiative submission and review.

Kaizen's P0 security PR #1 introduces random 30-minute `issue-invite` first login; dedicated P0 PR #2 creates PENDING accounts even with email/mobile blank. MFA enrollment still requires an accessible authenticator or independently accepted assisted worksite alternative.

## Data source/cutover rule
Use an HR-approved read-only export; compare expected staff counts/dept counts, missing/exited IDs and manager link validity. Stage independently and keep signed snapshot SHA-256. Do not import synthetic OrgAxis personnel to Kaizen. Do not assign HR/Director/Council/ADMIN from employee job titles.

Interim Kaizen CSV bridge: `tools/p0_hr_preflight.py` (separate repo), ensures unique employee numbers, valid parent references, no reporting cycles, no privileged CSV roles, and no private mobile exposure. It is validation only and **does not implement OrgAxis real-data ingestion**.

## Acceptance
- All pilot employees are present and match HR employee registry; 100% of pilot IDs must be real and active.
- Two-way reconciliation Kaizen `md_employee` <-> HR approved source; zero duplicate employee_no, zero unknown reviewer and zero unresolved reporting edge.
- Permissions: workers own submissions only; supervisors/managers only approved departments; council has explicit legal authority; no auto escalation from new job title.
- Audit employee activation, deactivation, transfer and reviewer scope change.
- Verify one real no-email/no-phone case with random invite, password setup, MFA or approved assisted alternative.
- HR owner signs data extract and update process; MIS signs privacy/backup controls.

**GO/NO-GO: NO-GO for PERSON-01 in production today.** Real HR source, import and reconciliation have not been executed.
