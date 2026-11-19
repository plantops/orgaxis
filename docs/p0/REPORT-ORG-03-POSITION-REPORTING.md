# OrgAxis P0 — Module ORG-03: Position & Reporting

Assessment date 2026-11-19. Source baseline: `plantops/orgaxis` Draft PR #12 + dedicated P0 branch.
**Engineering maturity: 3.0/5 (synthetic test pilot). Real-business-data acceptance: NOT ASSESSED.**

## Included functions verified in source
- Effective-dated `org_unit_relation`, `position_placement` and `position_reporting`; manager lookup and upward manager chain.
- `blankOrg(as_of)`: org unit and position snapshots; occupied vs unoccupied position display.
- `manager(person,date)`, `chain`, direct reports; cycle guard for changes to a position's LINE reporting.
- `changeReporting` closes old intervals and opens a new one; audit and change set history.
- Synthetic tests check 2026-07-01 warehouse reparent, manager before/after transfer, reporting loop rejection.
- Current v2 visualization can report managers via `/v2/org/full?as_of=...`.

Evidence: `schema.sql`, `go/main.go`, `go/workflow_v06.go`, `go/main_test.go`, `go/security_v06_test.go`.

## Critical limitations
- **Current `person` schema enforces `synthetic=1`** and the API tenant is hard-coded `tenant-cfc-pilot`. This is NOT a real company org chart. No real HR source data loaded.
- `position` rows are high-capacity shared templates, not one approved position slot per establishment. Vacancies, approved establishment and deputy/acting per-slot authority require additional work.
- OrgAxis `orgFull` handles primary reporting differently from `manager()` when acting supervisors exist; resolution inconsistency.
- Board of Members, Board of Supervisors, executive management, operations, Deputy and Foreman/Shift Leader are not yet represented in a verified full company dataset.
- Kaizen's `md_employee.report_to` is **reference-only** in the source and does not automatically control permission/approval. A CSV containing employee reporting cannot substitute for approved position-level delegated authority.

## Minimal production contract (P0 dependency, not implemented yet)
Approved source from HR: units, legal entity, position-slot identifier, title, slot holder, `LINE` reporting edges, valid_from/to, unit and approval scope. Stable HR employee number; no public PII export.

As-of reporting must resolve actual effective manager, deputy (only if written delegated scope), and correct owner for each Kaizen gate. Never derive privilege solely from job title or supervisor field.

## Gate test / acceptance
1. Import signed, non-demo HR org with effective dates into a schema that supports real records; separate it physically/logically from demo.
2. 100% valid references; unique slot IDs; no cycles; at most one published LINE supervisor per position/time; roots and governance defined.
3. Check 20 real department/reporting samples against signed HR chart, including at least one deputy, cross-department and shift leader.
4. Explain and approve each exceptional or ambiguous manager; verify transfers as-of and future effective changes.
5. Role/department permissions passed to Kaizen only through explicitly approved reviewer authorization mapping and recorded audit, not automatic role grant.
6. Business owner HR signs snapshot and effective date; MIS validates access controls; re-import diff/rollback tested.

**GO/NO-GO: NO-GO for asserting ORG-03 as production-ready today.** The source code demonstrates a temporal engine, not a verified live company reporting structure.

## P0 priority
1. Deliver approved `org_units.csv`, `position_slots.csv`, `reporting_edges.csv` as valid-time master data.
2. Implement real-data import/migration and dry-run in isolated DB, not by rewriting synthetic person rows.
3. Produce manager/RBAC review matrix for Kaizen 3 tracks and HR acceptance.
