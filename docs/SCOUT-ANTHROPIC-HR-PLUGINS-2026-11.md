# Scout: Anthropic Knowledge Work Plugins / Human Resources → OrgAxis

Date: 2026-11-19
Source: https://github.com/anthropics/knowledge-work-plugins/tree/main/human-resources
Source repo main tree SHA observed: b4c9a4e1de4b42868b74a6fe30ac766f64e43a3e
Plugin manifest version observed: 1.3.0
OrgAxis target baseline: Draft PR #12 v0.11, 35 entries/11 groups, engineering maturity mean 2.14/5, AppGate NOT_ASSESSED.

## Executive assessment

The upstream human-resources folder consists of nine Markdown workflow skill specifications, a README, connectors guide, plugin manifest, and MCP config. It is **not** an HRMS backend, relational data model, legal payroll engine, timeclock integration, optimizer, or enterprise security product. Do not claim that importing Markdown achieves an OrgAxis module implementation or AppGate compliance.

Main value: formalized HR decision-support workflows, consistent outputs, source citations, candidate/interview scorecards, headcount planning, performance calibration, and role-driven onboarding.

Connector limitations: `~~HRIS` and `~~ATS` are placeholders in `CONNECTORS.md`, not live integrations bundled by this plugin. Current `.mcp.json` has no HRIS or ATS server; Gmail and Google Calendar entries show empty endpoint URLs. Connector access requires a separately installed and authorized connector and its own data-access controls. The README lists legacy/narrative skill names (employee-handbook, compensation-benchmarking, people-analytics) that differ from the actual file names (policy-lookup, comp-analysis, people-report). Trust the skill files and manifest, not only the README.

License: repository root has Apache License 2.0; if redistributing adapted content, preserve notices and applicable license terms. This note independently summarizes concepts; no upstream SKILL.md copied into OrgAxis.

## Nine inspected upstream skills

| Upstream skill | Verified scope | OrgAxis destination | Priority | Decision |
|---|---|---|---|---|
| org-planning | Headcount, reporting spans, layers, org tradeoffs, hiring sequence, budget | ORG-02/03/04 + WFM-06 planning panel | P0 | Adapt with effective dates and critical-post demand |
| people-report | Headcount, turnover, org health, tenure, engagement, productivity | Org overview, PERSON-01 analytics | P0 | Adapt aggregate metrics with cohorts and provenance |
| policy-lookup | Search policies, cite policy section, exceptions | FLOW-02, employee self-service | P0 | Adapt source-cited, effective-date-aware policy retrieval |
| onboarding | Before day 1, day 1, week 1, 30/60/90 tasks, IT tools | PERSON-02 + TALENT-02 | P1 | Adapt for factory safety and qualification clearance |
| performance-review | Self-review, manager feedback, calibration, development | TALENT-03 | P1 | Adapt structured evidence and reviewer controls |
| recruiting-pipeline | Sourcing to accepted offer, stage velocities | **Candidate module** PERSON-05 Recruitment | P1 | Adapt only after headcount/position approval |
| interview-prep | Skill-based panel, question bank, scorecard | PERSON-05 Recruitment | P1 | Use position/JD competence version as source of rubric |
| comp-analysis | Benchmark bands, percentile, total compensation | PAY-01 / pay planning, separate from PAY-02 legal payroll | P2 | Advisory only; no market data bundled |
| draft-offer | Role, manager, location, compensation and offer document | PERSON-05 + PERSON-02 | P2 | Draft only from approved vacancy/pay band/terms |

Every upstream path is under `human-resources/skills/<name>/SKILL.md`.

## Recommendations specific to industrial OrgAxis

### A. Organization Planning Intelligence (P0)

Input: approved temporal org/position slots, function charter, RACI, vacancy, skills, contract-hour policy, shift coverage demand and actual employee availability.

Outputs:
- Span-of-control by manager, layers from root=0, empty leadership positions, oversized/small teams, unique essential skill coverage, succession/backup status.
- Reorg scenario as a **DRAFT**: cost, headcount, changed reporting edges, vacant posts, duty-owner conflicts and 24/7 coverage gaps.
- Effective-date diff; downstream authority/approval graph preview before publish.
- Headcount demand derived from post-hours, not generic white-collar span thresholds.

The upstream 5–8 direct-report and 4–6-management-layer benchmark values are generic heuristics in the Markdown skill, **not demonstrated safe standards for an industrial plant**. Make thresholds configurable and distinguish professional/managerial spans from shift leader/operator spans. Do not collapse Board of Members, Supervisory Board and CEO into one ordinary LINE hierarchy.

Candidate function names: `get_org_health(as_of,scope)`, `compare_org_versions(from,to)`, `simulate_reorg(change_set)`, `calculate_coverage_fte(post,period,policy)`.

### B. People Analytics (P0)

Baseline snapshot: hires/exits/FTE by unit, site, legal employer, position, shift and interval, absence/OT burden, critical-shift coverage, training validity.

Requirements: valid effective-time denominator, distinguish employees from position slots, voluntary vs involuntary exits if records permit, no mixing demo with real data, report denominator/sample size, provenance and permissions.

**Do not assign individual flight risk, promotion, disciplinary outcome or compensation based on demographic or sensitive attributes.** Aggregate small groups only under a privacy policy; avoid revealing individuals through sparse filters.

Candidate functions: `headcount_as_of`, `period_attrition`, `manager_spans`, `training_expiry`, `coverage_risk`.

### C. Versioned Policy Lookup (P0)

Sources: HR handbook, labor agreement, published company policies, position JD, site/shift/contract-specific rules and signed approval matrices.

Return: exact document title/version/effective date, section and excerpt, employee/site applicability, exception routing, missing-source status.

LLM retrieval is **advisory**. A source-cited answer cannot independently change leave balance, OT approval, statutory payroll, access rights or policies. Backend rule-pack + approved HR workflows remain authoritative.

### D. Industrial Onboarding (P1)

Original upstream is office-centric: buddy, laptop, calendar, 30/60/90 goals. For a factory add:
- Medical/fitness eligibility where legitimately required and controlled by authorized HR/HSE personnel (minimize sensitive health details).
- Initial HSE induction and exams, plant access authorizations, issued PPE, lockout/tagout or lifting permissions as applicable, equipment-specific training, shift leader signoff.
- Full separation of "employee hired", "employee allocated to shift", "employee qualified", and "employee authorized for hazardous work".
- Task gates to IT, HSE, department manager and HR; timestamp, evidence, expiration.

### E. Structured Performance/Recruitment (P1/P2)

- Versioned competency rubric bound to Job Description and actual position slot, calibrated reviewer scoring, bias checks and documented review.
- Recruiting pipeline cannot create a new role or headcount without approved position/requisition.
- Offers and compensation adjustments remain human-reviewed; compensation market percentile requires genuine dated source data and location comparability.
- Do not copy default US-centric equity, PTO, at-will/exempt terminology or compensation assumptions into Vietnamese employment rules.

## Implementation approach: lean, not nine new pages

Implement **one read-only HR Insights workspace** and **one action-oriented HR Case/Tasks workflow**, reusing approved OrgAxis records. New functionality should be delivered as composable backend services and screens with existing role views.

1. P0: Org health and workforce analytics (source-based, no model access required), temporal as-of and configurable warning thresholds.
2. P0: versioned policy retrieval with source section, applicability and citations. Separate private HR policies from public employee guidance.
3. P1: JD/competency-aligned onboarding and performance-review/case templates with approval evidence.
4. P1: recruitment requisitions and pipeline, using position slots and capex/OPEX approval, with candidate privacy isolation.
5. P2: compensation advisory and offer drafting after verified authoritative datasets are available.

Do not add nine top-level navigation modules. Proposed v0.12 intake changes only:
- ORG-07: Org Planning & Org Health, consumes ORG-02/03/04 and WFM-06.
- PERSON-05: Recruiting & Onboarding, link PERSON-02, TALENT-01/02.
- FLOW-03 (or shared KNOW service): Versioned HR Policy Lookup.
- Expand TALENT-03 with structured calibration, and PAY-01 with advisory pay-band analysis. These are **candidates**, not additional v0.11 implemented modules.

## Acceptance tests before maturity promotion

- Historical org snapshot and spans reproducible at as_of before/after structural change; no duplicate person count across multiple assignments.
- 24/7 coverage demand accounted separately from contracted weekly 40–42h per worker; leave/training and hours rules affect feasibility, not merely headcount.
- Org plan cannot publish if reporting cycle, unresolved critical post, conflicting accountable owner, separation-of-duties breach or unapproved rights transfer.
- Policy query must return document/version/section/correct effective-date match; if not found, return explicit uncertainty; no invented policy or unsupported approval.
- Staff without relevant role must not see candidate CV, salary bands, individual disciplinary history or health details; audit reads of sensitive documents.
- Onboarding incomplete HSE/training/permission blocks role-specific dispatch and does not automatically grant permissions merely from a checklist.
- Performance rubric consistent across candidates within same approved vacancy; automated assistant only drafts, HR-approved final evaluation retained with reviewer/evidence.
- Test with 1,000 synthetic profiles; real-data cross-check, regression, security and independent AppGate UAT still required.

## AppGate and deliverable boundary

Mappings: BIZ-01/02/03/04/06; DAT-01/02/03/06; SEC-02/04/05; INT-01/02/05; UX-01/04; GOV-01/02/03/04. No framework criterion is marked PASS solely because a Markdown workflow exists. Source-only discovery does not alter OrgAxis v0.11 35-module scores (2.14/5) or AppGate NOT_ASSESSED status.

## Primary source links

- https://github.com/anthropics/knowledge-work-plugins/tree/main/human-resources
- https://github.com/anthropics/knowledge-work-plugins/blob/main/human-resources/README.md
- https://github.com/anthropics/knowledge-work-plugins/blob/main/human-resources/CONNECTORS.md
- https://github.com/anthropics/knowledge-work-plugins/blob/main/human-resources/.mcp.json
- https://github.com/anthropics/knowledge-work-plugins/blob/main/LICENSE
- Nine individual `human-resources/skills/*/SKILL.md` files inspected on 2026-11-19

## Related user-supplied KB (2026-11-19)

OrgAxis also preserves a complete user-supplied reference on mixed-workforce onboarding/2FA: [`docs/kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md`](kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md). The separate engineering/security review is [`docs/kb/ONBOARDING-IDENTITY-2FA-ORGAXIS-INTEGRATION.md`](kb/ONBOARDING-IDENTITY-2FA-ORGAXIS-INTEGRATION.md). This covers workers without email/phone, TOTP, supervised enrollment, kiosk, proxy and actor/subject audit. The supplied source proposes a date-of-birth-based first password and static QR login; both are rejected as standalone authentication designs. No existing module score is upgraded.
