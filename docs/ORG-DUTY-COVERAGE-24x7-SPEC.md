# OrgAxis v0.10 — Organization Function, Duty Charter, Job Description and 24/7 Coverage

Status: DESIGN CONTRACT / implementation backlog, not running features, not approved HR policy.
Baseline: feature/p0-attendance-ledger-lock-v09 (4e2f9c0); branch: feature/org-duties-coverage-24x7-v10.
Context: anonymous/pseudonymous corporate hierarchy, cement plant, one or multiple sites, 365 days/year, critical posts staffed continuously, most individual workers nominally 40–42 hours/week.

## Domain decisions

1. Separate legal company / organization hierarchy (org units), management/reporting hierarchy (position slots and reporting edges), business functions (what must be delivered), responsibility charter (who is accountable), and coverage schedule (who is on duty). These are not the same tree.
2. Root depth 0 and arbitrary depth for LINE tree. Depth is derived, never encoded by titles. Board oversight, functional, acting and delegated authority are separately typed effective-dated links; not inferred from Deputy/Vice title.
3. A function belongs to a catalog independent of organizational unit or incumbent. A business-process activity has exactly one primary accountable organizational owner per site/scope and effective instant. Responsible executors may be plural, and C/I/support owners may overlap. SoD-required roles cannot be fulfilled by the same person for a transaction.
4. Functional responsibility changes are approved and effective-dated: draft -> reviewed -> approved -> published; historical as-of reads must remain stable. Reorganization must not silently transfer regulated or safety-critical authorization.
5. Standard job description = job family template + position slot-specific requirements + strictly dated per-person temporary assignments. Never clone one employee-specific job description for every person.
6. The plant operates 24/7; only designated operational functions/posts need continuous manned coverage. Others use business-hours, scheduled inspections, on-call or event-driven coverage, explicitly differentiated.
7. Schedules are governed by an approved policy effective date, site, legal employer and worker category. 40–42 nominal weekly hours are company targets, NOT statutory legal classifications or automatically permitted overtime.
8. This feature must not treat a position, skill, or scheduled employee as authorization to operate a hazardous device: training, required permits, physical presence and operating authorization are distinct evidence.

## Core entities (new additive tables; do not overwrite existing assignment/position)

| Entity | Important fields | Purpose |
|---|---|---|
| oa_business_function | function_id, code, parent_function_id, description, outcome, KPI, criticality, active interval | Atomic business function / responsibility |
| oa_function_owner | function_id, scope/site, accountable_unit_id, support_unit_ids, validity, authority_ref, change_set_id | Who owns deliverable, independent of staff |
| oa_function_raci | function_id, activity_id, unit_or_position_id, R/A/C/I, valid_from/to | Cross-functional RACI |
| oa_job_profile_version | job_profile_id, revision, duties, deliverables, capability requirements, safety requirements, validity | Standard job description |
| oa_position_duty | position_id, function_id, activity_id, primary_or_backup, workload, effective dates | Duty of a particular approved position slot |
| oa_person_duty | person_id, position_id, duty_id, assignment_type, scope, start/end, approved_by, evidence_id | Temporary task/acting duty, no permanent title changes |
| oa_coverage_post | post_id, site, function_id, physical_location, skill/authorization requirements, minimum/desired people, coverage_mode, valid_from/to | Coverage demand independent of positions |
| oa_coverage_window | post_id, recurring calendar / ad hoc window, start/end, min_people, criticality | 24/7, business-hours, seasonal, on-call, event-driven windows |
| oa_shift_pattern | site, period, start/end local, timezone, paid break rule, handover rule, validity | Shift pattern template |
| oa_roster_assignment | employee, post, shift_start/end, planned minutes, schedule revision, approval status | Planned person-to-post coverage; link existing time roster |
| oa_coverage_event | post, time range, planned/actual, staffed count, exception and cause | Coverage gap/overlap evidence |
| oa_shift_handover | from/to employees, post, timestamp, open tasks, equipment/permit state, accepted_at | Accountable shift transition |
| oa_duty_change_set | draft/approved/published, actor, reason, effective date, diff, evidence, audit | Governance of charter/job changes |

Tables are conceptual, not yet migrated. Use canonical employee IDs and time events from existing OrgAxis; do not create duplicate personnel or payroll records.

## Invariants and conflict rules

- For each (function, site, scope, instant) exactly one published A is effective. Many R/C/I allowed, but conflicting published A intervals are rejected; exception requires narrower explicit scope and two-person signoff.
- A position LINE parent at an instant is 0 or 1; no cycles; functional and oversight edges do not silently alter LINE.
- Duty overlap is allowed when explicitly collaborating, cross-training or conducting handover; decision/approval authority and transaction segregation are separately enforced.
- A person cannot fulfill physically incompatible simultaneous posts. Planned training, paid leave, shift attendance and field assignment must be checked together; overlapping roles that do not consume exclusive physical presence may be permitted by rule and capacity.
- Deputy/acting positions require explicit effective-dated substitution plus delegated permission; job title does not grant approval authority.
- Qualification validity, mandatory training and physical availability gate safety-relevant dispatch.
- Scheduled critical coverage gaps create exception and escalation, not silent auto-approval of overtime.
- Never mutate approved historical rosters and locked time ledger; use approved correction and provenance chain instead.
- All changes have tenant/site/actor, source_ref, effective interval and transaction-time audit.

## Cement plant mapping examples (illustrative; not actual Chinfon reporting)

| Function | Primary accountable | Executors | Consulted / independent gates | Coverage |
|---|---|---|---|---|
| Jetty vessel unloading | Warehouse / Port Manager | Foreman (= shift leader), crane operator, tallyman | QC, HSE, maintenance | According to vessel/port operating window |
| Material receipt and stock custody | Warehouse Manager | Storekeeper, tallyman | QC for incoming acceptance | Shift/calendar driven |
| Kiln running operation | Production/Kiln Manager | CCR operator, patroller, foreman | Maintenance/HSE | Continuous as required |
| Sampling and QC release | Quality Manager | Sampler, laboratory team | Operations/warehouse | According to QC duty plan, possibly 24/7 |
| Electrical/mechanical emergency response | Maintenance Manager | Technician, on-call roster | Operations permit owner | Shift or on-call depending asset risk |
| Payroll approval and release | HR/Payroll authorized owners | HR records and payroll specialists | Finance/audit SoD | Periodic; not continuous |

Operations may execute unloading, but QC must remain the independent quality decision maker under approved policy. RACI does not replace legal signature authority, process safety roles or individual work permits.

## 24/7 coverage arithmetic and feasibility

For ONE post requiring exactly one person continuously:
- Demand = 24 x 7 = 168 person-hours/week, before simultaneous handover, break relief, skills or leave.
- Theoretical net FTE lower bound = 168 / 40 = 4.20, or 168 / 42 = 4.00.
- At 80% effective availability, nominal capacity = 32 h/worker/week (40 x .8), giving 168 / 32 = 5.25, rounded up to 6 workers. The 80% assumption is illustrative, not measured company availability.
- Under three 8h shifts, there are 21 one-person shifts per week. An example five-person allocation of [5,4,4,4,4] gives [40,32,32,32,32] hours on-post, totaling 168; if all five have 40-hour weekly contracts, the remaining 32 paid hours must be assigned to real other duties, cross-coverage, training or measured handover, not manufactured punches or imaginary work.
- Five employees alone do not prove annual coverage with weekly rest, leave, illness and required role qualification; pool or dedicated relief may be necessary. Six is merely an illustrative planning result from the availability assumption.
- Use time-based workload, not simplistic count of employees, for posts requiring simultaneous operators or distinct skills.

The optimization must account for individual contracts, policy-dependent normal hours, consecutive rest, shift-change rest, break coverage, public holidays, annual leave, training, night duty, overtime approval, and employee consent. Keep workload thresholds and legal rules versioned and reviewed by HR/legal. Reference: Vietnam Labor Code integrated text 18/VBHN-VPQH (2026), Articles 105, 107, 109–111; exact company policy and any special-work regulation require separate review.

## Planning engine contract

Input: as_of date range, site, duty/coverage requirements, approved employee pool, qualifications/authorization, approved leave and absences, employee normal hours policies, team preferences, peak workload, devices and shifts.

Output: coverage calendar at 15–60 minute resolution, uncovered minutes per critical post, roster hours per person, normal/OT split (rule-pack-driven), rest violations, approval-needed assignments, flexible/cross-trained staff, handover continuity, and estimated relief FTE.

Scheduling algorithm must treat hard constraints (hours/rest/qualification/SoD/physical double booking) separately from soft preferences (fair rotations, night-duty equity, preference, overlap for training). An uncovered post remains explicitly uncovered if no valid assignment exists.

## Proposed APIs (versioned, authenticated, tenant+site scope)

GET /v2/org/functions?as_of=YYYY-MM-DD
GET /v2/org/responsibility?unit_id=...&as_of=...
GET /v2/positions/{id}/job-description?as_of=...
GET /v2/people/{id}/duties?as_of=...  (own, authorized supervisor, HR)
POST /v2/org/duty-change-sets/validate  (dry-run; conflict report)
POST /v2/org/duty-change-sets  (HR/org owner; draft)
POST /v2/org/duty-change-sets/{id}/publish  (independent authorized approver)
GET /v2/coverage/demand?site=...&from=...&to=...
POST /v2/coverage/roster/validate  (strict check; no implicit commit)
POST /v2/coverage/roster/publish  (authorized scheduler)
GET /v2/coverage/gaps?site=...&from=...&to=...
POST /v2/coverage/handover  (valid actor and shift; append event)

All POST mutations must enforce server-side authorization, CSRF, idempotency, audit and transactional integrity.

## UI

1. Org Explorer: unit tree + expandable Function Charter (mission, responsibilities, KPIs, owner, boundary, dependencies).
2. Position detail pane: versioned job description, deliverables, qualifications, coverage demand, primary/deputy/backup, person history.
3. Person pane: actual approved duties, temporary assignment, standard hours, training/qualification, schedule and planned leave; visibility by role.
4. Responsibility Matrix: function x unit RACI with duplicate-A and orphan-function warnings, effective-date comparison.
5. Coverage Planner: weekly 24x7 heatmap (required / staffed / gap), shift calendar, staffing load; show only roles that truly require continuous coverage.
6. Exception queue: missing role, duplicate accountable owner, no relief, missed handover, invalid qualification, overtime/rest conflicts.

## Acceptance test scenarios

- Same function + site/time with two published A owners is rejected; multiple R accepted.
- Authorized merger/split creates versioned owner change at boundary date, historical view unchanged.
- Foreman/shift leader substitution does not automatically grant payroll or safety approval.
- For 21 x 8h slots, planned coverage sums to 168 hours exactly before overlaps; report gap if one of three shifts per day is uncovered.
- A worker already committed to one exclusive physical post is rejected for another overlapping post.
- Planned weekly hours exceeding a worker's configured normal-hours policy do not silently become normal hours.
- Invalid rest/shift-change gaps, denied OT and missing qualifications create blockers.
- 24/7 demand is present across all calendar dates, including Sundays and public holidays; personal normal-hours, night, rest and OT rules remain separately enforced.
- Leave, sickness and training recalculate available capacity and uncovered-post risk without modifying approved raw punches.
- Import anonymized position/duty data through preview -> validation -> approve -> publish with transactional rollback on a conflict.
- Tests include multi-role RBAC/CSRF, SQL conflict constraints, temporal boundaries and 1,000 synthetic workforce load.

## Delivery order and AppGate

P0: function charter + job description + function ownership (temporal, A uniqueness) and 24x7 coverage demand/read-only gap metrics, with schema migration and tests.
P1: shift scheduling, labor-hours/rest/qualifications, exceptions, handover, absence/OT links, approval and audit.
P2: scenario simulation, headcount/relief forecasts, cross-training, resource optimization and workforce analytics.

AppGate mapping: BIZ-01/02/03/04/06; DAT-01/02/03/04/05; SEC-02/04; INT-01/02; REL-02/04/05; DEL-02/03; UX-01/04; GOV-01/02/03/04. Current PR is specification-only; no criteria pass or UAT approval claimed.

## Explicit non-goals

- No implication every department must staff 24/7.
- No automatic permission to exceed individual 40–42h targets.
- No synthetic salary numbers treated as official pay.
- No claim that masked IDs by themselves are anonymous personal data.
- No automatic grants from org chart, deputy title, duty assignment or simulated certificate.
