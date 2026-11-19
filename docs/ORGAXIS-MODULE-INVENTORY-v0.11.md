# OrgAxis v0.11 — Kiểm kê đầy đủ module, function và maturity /5

Ngày snapshot: 2026-11-19. Baseline: v0.10 design `475b2c977f18cb8e505d803eecd94c7f025f3c4a`. V0.11 adds the catalog API, secure workspace inventory view and tests; it **does not** implement missing business domains.
Source registry: `go/module_catalog_v11.go`. API: ADMIN/HR/DIRECTOR authenticated `GET /v2/modules`; optional `group=ORG|PERSON|TALENT|FLOW|WFM|PAY|SEC|INT|PLAT|UX|GATE`, `status=operational_demo|partial_demo|design_only|not_implemented`.

## Scope and score interpretation

- Scale: 0 absent; 1 concept/spec; 2 partial functional demo; 3 source-tested synthetic pilot; 4 tested against representative real devices/data with reconciliation and operational failure tests; 5 independent acceptance and stable production operations.
- Intermediate 0.5 increments are engineering judgement. These are **not** authoritative AppGate criterion scores.
- `operational_demo` means testable pilot code, not company-production operation.
- `design_only` has no executable domain function even if a proposed API appears in documentation.
- Numeric average is unweighted, for inventory prioritization only, not an enterprise go-live readiness or AppGate weighted score.
- `main` has not absorbed the stacked PRs. Baseline includes draft changes PR #1..#11. Code/CI can be green while actual device, HR data and UAT remain absent.
- Maturity of each item should be reviewed again after tests, HR policy validation, actual device tests and AppGate UAT.

### Group maturity snapshot

| Group | Scope | Modules | Average /5 |
|---|---|---:|---:|
| ORG | Cơ cấu, chức năng và vị trí | 6 | 2.00 |
| PERSON | Hồ sơ, vòng đời, pháp lý nhân sự | 4 | 2.38 |
| TALENT | Năng lực, đào tạo, hiệu suất | 5 | 2.30 |
| FLOW | Workflow và nghỉ phép | 2 | 2.75 |
| WFM | Lịch ca, công, phủ 24/7 | 6 | 2.33 |
| PAY | Payroll, BHXH, PIT | 3 | 1.33 |
| SEC | Identity, audit, privacy | 2 | 2.50 |
| INT | Tích hợp và nạp dữ liệu | 2 | 1.00 |
| PLAT | Windows, SQLite, vận hành | 3 | 2.50 |
| UX | Giao diện đa vai trò | 1 | 3.00 |
| GATE | Đánh giá AppGate | 1 | 1.00 |
| **ALL** | **All identified capabilities** | **35** | **2.14** |

## Complete module/function ledger

| ID | Module | /5 | Code status | Function inventory |
|---|---|---:|---|---|
| ORG-01 | Pháp nhân và tenant | 2.0 | partial_demo | Danh sách/tạo công ty; tenant profile; corporate relationship |
| ORG-02 | Cấu trúc đơn vị theo thời gian | 3.0 | operational_demo | Đọc cây đơn vị; thay đổi cha-con theo ngày; lịch sử |
| ORG-03 | Vị trí, tuyến báo cáo, biên chế | 3.0 | operational_demo | Tra manager; chain; direct reports; move; cycle guard |
| ORG-04 | Change-set tái cơ cấu | 2.0 | partial_demo | Ghi nhận change; tra lịch sử; backdated read |
| ORG-05 | Function Charter & RACI | 1.0 | design_only | Chức năng khối/phòng; RACI; SoD; conflict detection |
| ORG-06 | Job Description & duty từng người | 1.0 | design_only | JD version; nhiệm vụ vị trí; nhiệm vụ cá nhân; deputy/acting |
| PERSON-01 | Hồ sơ và lịch sử công việc | 3.0 | operational_demo | Tìm kiếm; xem dossier; employment/assignment timeline |
| PERSON-02 | Hire/transfer/separation | 2.5 | partial_demo | Hire; move; separation; HR events |
| PERSON-03 | Lịch sử nghề và điều kiện lao động | 2.0 | partial_demo | Tra thời gian nghề; lịch sử tiếp xúc/điều kiện; source ref |
| PERSON-04 | Chuyên gia nước ngoài | 2.0 | partial_demo | Tra giấy tờ; quyền làm việc theo record; hạn 90 ngày |
| TALENT-01 | Kỹ năng và điều kiện giao việc | 2.5 | partial_demo | Danh mục; assessment; holder search; skill-only check |
| TALENT-02 | Đào tạo, chứng chỉ | 2.5 | partial_demo | Chương trình; kết quả; học viên; ngày hết hạn |
| TALENT-03 | KPI và đánh giá hiệu suất | 2.0 | partial_demo | KPI theo kỳ; mục tiêu/thực hiện; nhập kết quả |
| TALENT-04 | Khen thưởng | 2.5 | partial_demo | Mở proposal; tính bậc demo; tuyến duyệt; lịch sử |
| TALENT-05 | Kỷ luật và hồ sơ vụ việc | 2.0 | partial_demo | Mở case; cấp xét duyệt; audit |
| FLOW-01 | Quy trình phê duyệt theo tuyến | 3.0 | operational_demo | Tuyến 1–n; case create/decision/resubmit; no self-approval |
| FLOW-02 | Nghỉ phép và vắng mặt | 2.5 | partial_demo | Xin nghỉ; xét duyệt; lịch sử; leave credit model |
| WFM-01 | Mẫu ca và roster | 2.5 | partial_demo | Tra roster; day plan per person/month; shift match |
| WFM-02 | Thiết bị và raw punches | 3.0 | operational_demo | Device/badge binding; pull page; exception; append-only triggers |
| WFM-03 | Tính công và exception | 3.0 | operational_demo | IN/OUT; ca; duplicate; missing; OT/leave eval |
| WFM-04 | Khóa kỳ công | 2.5 | partial_demo | Calendar import; readiness; prepare; approve; verify drift |
| WFM-05 | OT và chứng cứ phê duyệt | 2.0 | partial_demo | Approved OT time segment; pay preview gating |
| WFM-06 | Coverage 365/24/7 và bàn giao | 1.0 | design_only | Coverage demand; roster optimization; gap report; handover |
| PAY-01 | Bảng lương demo và gross preview | 2.0 | partial_demo | Payroll year/person; workbench; payslip; PIT simulation demo |
| PAY-02 | Lương pháp lý VN, PIT/BHXH | 0.5 | not_implemented | Versioned rules dự kiến; payslip thật; export ngân hàng |
| PAY-03 | Phúc lợi và bảo hiểm hồ sơ | 1.5 | partial_demo | Claim create/list; coverage status; tax profile record |
| SEC-01 | Định danh, tài khoản, RBAC | 3.0 | operational_demo | Login/logout; user create; self/manager payroll read |
| SEC-02 | Audit và riêng tư | 2.0 | partial_demo | Audit events; role-scoped views; ledger trace |
| INT-01 | Adapter máy công / ERP | 1.5 | partial_demo | Vendor-neutral fetch; cursor; error sanitization |
| INT-02 | Import HR/Org khuyết danh | 0.5 | not_implemented | Import structure, slots, edges, function charter dự kiến |
| PLAT-01 | Windows service/tray và artifact | 3.0 | operational_demo | GUI/server EXE; installer; launch; single-writer lock |
| PLAT-02 | SQLite migrations/backup/restore | 2.5 | partial_demo | v06–v09 schema migrations; snapshots; locked rows |
| PLAT-03 | Logs/health/diagnostics | 2.0 | partial_demo | Health; bounded logs; uptime; LAN bearer |
| UX-01 | Workspaces và overview-to-detail | 3.0 | operational_demo | Workspace; org as-of; person pane; time and payroll demo |
| GATE-01 | AppGate evidence và UAT | 1.0 | design_only | AppGate rubric mapping; PR docs; CI logs |

## Evidence, limitations and upgrade requirements

### ORG-01 — Pháp nhân và tenant (2.0/5, partial_demo)

Scope: Khai báo pháp nhân, metadata tenant; phạm vi demo một tenant.

Source: `go/main.go`, `schema.sql`.

Gap: Cách ly tenant thực; legal-entity operations.

### ORG-02 — Cấu trúc đơn vị theo thời gian (3.0/5, operational_demo)

Scope: Blank organization, unit tree, as-of, chuyển Warehouse mẫu.

Source: `go/main.go`, `go/main_test.go`, `seed.py`.

Gap: Tạo/đóng/gộp/tách và approval workflow.

### ORG-03 — Vị trí, tuyến báo cáo, biên chế (3.0/5, operational_demo)

Scope: Position placement, reporting, holder limit, manager chain, đổi tuyến.

Source: `go/main.go`, `go/main_test.go`, `go/security_v06_test.go`.

Gap: Position slots độc lập; acting/dual reporting; vacancy.

### ORG-04 — Change-set tái cơ cấu (2.0/5, partial_demo)

Scope: Có change_set và event mô phỏng, chưa đủ vòng đời phê duyệt.

Source: `schema.sql`, `seed.py`, `go/main.go`.

Gap: Draft/review/approve/publish; prospective merger/split.

### ORG-05 — Function Charter & RACI (1.0/5, design_only)

Scope: Đặc tả đầu mối A duy nhất theo chức năng và phạm vi.

Source: `docs/ORG-DUTY-COVERAGE-24x7-SPEC.md`.

Gap: Chưa có bảng, API, validation, UI.

### ORG-06 — Job Description & duty từng người (1.0/5, design_only)

Scope: Đặc tả job family, vị trí, nhiệm vụ kiêm nhiệm và thay thế.

Source: `docs/ORG-DUTY-COVERAGE-24x7-SPEC.md`.

Gap: Chưa có migration, quyền cập nhật, lịch sử nhiệm vụ.

### PERSON-01 — Hồ sơ và lịch sử công việc (3.0/5, operational_demo)

Scope: Person, employment, position history; API hồ sơ phân quyền.

Source: `go/modules.go`, `go/workflow_v06.go`, `go/security_v06_test.go`.

Gap: Hồ sơ pháp lý và PII thật; import source of record.

### PERSON-02 — Hire/transfer/separation (2.5/5, partial_demo)

Scope: Có hire demo, chuyển vị trí, kết thúc việc theo ngày.

Source: `go/workforce.go`, `go/main.go`, `go/modules_test.go`.

Gap: Hợp đồng; rehire; pháp nhân khác; phê duyệt thực.

### PERSON-03 — Lịch sử nghề và điều kiện lao động (2.0/5, partial_demo)

Scope: Có occupation periods và nhóm điều kiện synthetic.

Source: `schema.sql`, `go/modules.go`.

Gap: Thẩm định quy tắc độc hại, phụ cấp, hồ sơ xác minh.

### PERSON-04 — Chuyên gia nước ngoài (2.0/5, partial_demo)

Scope: Masked travel/immigration/authorization demo.

Source: `go/enterprise_modules.go`, `go/modules_test.go`.

Gap: Workflow pháp lý, cảnh báo, nguồn chứng từ.

### TALENT-01 — Kỹ năng và điều kiện giao việc (2.5/5, partial_demo)

Scope: Đánh giá kỹ năng L1–L4, kiểm tra điều kiện kỹ năng, không cấp phép vận hành.

Source: `go/modules.go`, `go/workforce.go`, `go/modules_test.go`.

Gap: Kỹ năng theo thiết bị; giấy phép; hạn chứng chỉ; HSE gates.

### TALENT-02 — Đào tạo, chứng chỉ (2.5/5, partial_demo)

Scope: Training catalog, record, thời hạn hiệu lực; chứng chỉ chưa được cấp tự động.

Source: `go/modules.go`, `go/modules_test.go`.

Gap: Kế hoạch đào tạo; bắt buộc theo vị trí; hồ sơ phê duyệt.

### TALENT-03 — KPI và đánh giá hiệu suất (2.0/5, partial_demo)

Scope: Lưu và đọc KPI result synthetic.

Source: `go/modules.go`, `go/modules_test.go`.

Gap: Mẫu đánh giá; người duyệt; liên kết nguồn nghiệp vụ.

### TALENT-04 — Khen thưởng (2.5/5, partial_demo)

Scope: Đề xuất, review HR, phê duyệt Director; demo Kaizen tiers.

Source: `go/workflow_v06.go`, `go/security_v06_test.go`.

Gap: Quy chế thưởng thực; chứng từ; hạch toán.

### TALENT-05 — Kỷ luật và hồ sơ vụ việc (2.0/5, partial_demo)

Scope: Luồng trình duyệt sự việc; không tự ra quyết định kỷ luật pháp lý.

Source: `go/workflow_v06.go`, `go/modules.go`.

Gap: SoD/Legal, hồ sơ, privacy và luật lao động.

### FLOW-01 — Quy trình phê duyệt theo tuyến (3.0/5, operational_demo)

Scope: LINE_MANAGER theo as-of, escalation leave, role-based case approval.

Source: `go/workflow_v06.go`, `go/security_v06_test.go`.

Gap: Deputy/acting/delegation/functional routing; SoD phức tạp.

### FLOW-02 — Nghỉ phép và vắng mặt (2.5/5, partial_demo)

Scope: Có case, phê duyệt và demo counting; chưa gắn đầy đủ vào time close.

Source: `go/modules.go`, `go/workflow_v06.go`, `go/attendance_close_v09.go`.

Gap: Quota/accrual, lễ, ca, entitlement, evidence binding.

### WFM-01 — Mẫu ca và roster (2.5/5, partial_demo)

Scope: 6.000 ca synthetic, overnight và mẫu SHIFT/REST/HOLIDAY.

Source: `go/attendance_v07.go`, `go/attendance_close_v09.go`, `go/attendance_seed_v07.go`.

Gap: Rotation, đổi ca, 40–42h policy, rest và holidays được xác nhận.

### WFM-02 — Thiết bị và raw punches (3.0/5, operational_demo)

Scope: Import idempotent/quarantine; simulated JSON pull; checkpoint giao dịch.

Source: `go/attendance_v07.go`, `go/attendance_checkpoint_v08.go`, `go/attendance_checkpoint_v08_test.go`.

Gap: Máy thực; event provenance; auto-poll, offline 8h, gateway.

### WFM-03 — Tính công và exception (3.0/5, operational_demo)

Scope: Pure-Go event pairing, ngày công và anomaly; synthetic unit tests.

Source: `go/timecore/engine.go`, `go/timecore/engine_test.go`, `go/attendance_v07_test.go`.

Gap: Golden attendance data; site policy; correction approval.

### WFM-04 — Khóa kỳ công (2.5/5, partial_demo)

Scope: Plan 28–31 ngày, readiness blockers, HR prepare/PAYROLL lock, source digest.

Source: `go/attendance_close_v09.go`, `go/attendance_close_v09_test.go`.

Gap: Batch close, correction/reopen, leave OT binding, real UAT.

### WFM-05 — OT và chứng cứ phê duyệt (2.0/5, partial_demo)

Scope: Có model interval approved và tính thử; chưa chứng minh case linkage.

Source: `go/attendance_v07.go`, `go/timecore/engine.go`.

Gap: VN rule pack, consent, limits, approved case provenance.

### WFM-06 — Coverage 365/24/7 và bàn giao (1.0/5, design_only)

Scope: Đặc tả coverage post, staffing gap, duty handover, relief FTE.

Source: `docs/ORG-DUTY-COVERAGE-24x7-SPEC.md`.

Gap: Chưa có implementation, scheduler, heatmap.

### PAY-01 — Bảng lương demo và gross preview (2.0/5, partial_demo)

Scope: 12k payslips synthetic độc lập time engine; gross preview blocked khi thiếu nguồn.

Source: `go/payroll_workbench.go`, `go/workflow_v06.go`, `go/attendance_v07.go`.

Gap: Source-to-pay; reconciliation; approved month integration.

### PAY-02 — Lương pháp lý VN, PIT/BHXH (0.5/5, not_implemented)

Scope: Chưa có engine tính lương, PIT, BHXH được HR/Finance đối soát.

Source: `docs/HRM-BENCHMARK-2026.md`, `go/README-v09.md`.

Gap: Tất cả logic pháp lý, UAT và chi trả thực.

### PAY-03 — Phúc lợi và bảo hiểm hồ sơ (1.5/5, partial_demo)

Scope: Benefit claim registry, SI periods, tax profiles ở mức record.

Source: `schema.sql`, `go/enterprise_modules.go`.

Gap: Entitlement engine, tính đóng/hưởng, quyết toán.

### SEC-01 — Định danh, tài khoản, RBAC (3.0/5, operational_demo)

Scope: Local personal login, session, CSRF, roles, scope test.

Source: `go/security_v06.go`, `go/security_v06_test.go`.

Gap: SSO/OIDC MFA, service accounts, legal employer scopes.

### SEC-02 — Audit và riêng tư (2.0/5, partial_demo)

Scope: Actor audit cho workflow; source digest/trigger raw; chưa đủ PII governance.

Source: `go/security_v06.go`, `go/attendance_close_v09.go`, `go/workflow_v06.go`.

Gap: Privacy inventory, tamper-evidence, retention, export/delete.

### INT-01 — Adapter máy công / ERP (1.5/5, partial_demo)

Scope: HTTP JSON contract máy công, chưa có ERP live hoặc vendor gateway.

Source: `go/timecore/adapter.go`, `go/attendance_checkpoint_v08.go`.

Gap: Thiết bị và ERP thực; OpenAPI; reconciliation.

### INT-02 — Import HR/Org khuyết danh (0.5/5, not_implemented)

Scope: Seed Python, chưa có pipeline CSV import review/approve/merge.

Source: `seed.py`, `docs/ORG-DUTY-COVERAGE-24x7-SPEC.md`.

Gap: CSV validation, preview, dry-run, approval, rollback.

### PLAT-01 — Windows service/tray và artifact (3.0/5, operational_demo)

Scope: Go cross-build Win10 x64, tray/service scripts, singleton test.

Source: `go/runtime.go`, `go/windows/Install-OrgAxis-Service.ps1`, `go/ops_test.go`.

Gap: UAT Windows thật; service recovery; code signing.

### PLAT-02 — SQLite migrations/backup/restore (2.5/5, partial_demo)

Scope: Additive upgrades và VACUUM INTO pre-upgrade snapshots.

Source: `go/runtime.go`, `go/attendance_backup_v07.go`, `go/attendance_close_v09.go`.

Gap: Migration ledger, restore drill, off-host backups.

### PLAT-03 — Logs/health/diagnostics (2.0/5, partial_demo)

Scope: Rotating log, status, health, limited LAN TLS read-only.

Source: `go/ops.go`, `go/ops_test.go`, `go/main.go`.

Gap: Alerts, metrics, SLO, incident runbook.

### UX-01 — Workspaces và overview-to-detail (3.0/5, operational_demo)

Scope: Org explorer 1k nodes, person detail, leave/bonus/discipline, time lab; nhiều UI cũ.

Source: `go/assets/workspace.js`, `go/assets/app.js`, `go/assets/time.js`.

Gap: Task UAT, module navigation unify, WCAG/device.

### GATE-01 — AppGate evidence và UAT (1.0/5, design_only)

Scope: AppGate intake và evidence map, chưa có independent score.

Source: `docs/APPGATE-HR-READINESS.md`, `plantops/app-gate/framework/enterprise-v2026.11.2.yaml`.

Gap: Actual signed assessment, evidence coverage, all hard gates.


## v0.11 delivered (source-scope only)

1. Maintainable single-source Go module catalog containing group, maturity, status, function list, evidence and gaps; 35 entries in 11 groups.
2. Role-scoped read-only API `GET /v2/modules`, accessible only to authenticated ADMIN/HR/DIRECTOR demo users, no personal salary/HR records exposed; invalid group/status gives HTTP 422.
3. HR Workspace nav "Module Inventory · v0.11"; overview counts, mean maturity and all 11 grouped tables.
4. Linux CGO automated tests (added but not yet re-executed successfully in CI) check auth, shape, IDs, filter semantics, missing critical modules, score bounds and no false promotion of design-only functionality.
5. Documentation explicitly separates engineering maturity from 49-criterion AppGate UAT scoring.

## P0 — Functional reliability and source-of-truth

**Priority A — Preserve correctness:** Finish device-specific, read-only time-clock ingestion; actual 8h disconnection replay, person/badge mapping, authoritative shift/leave/OT approvals and immutable correction/retro workflow. Link full month attendance ledger to real company-approved payroll rules and independent reconciliation (at least 100 persons x 2 periods). No law/payroll claim from synthetic data.

**Priority B — Build temporal organization authoring:** Import anonymized unit/position slot/reporting via CSV; dry-run, scope/foreign key/cycle/historical overlap check, review/publish, independent change history; support Board, Board of Supervisors, CEO, Deputy, Director, Manager, Supervisor, Foreman/Shift Leader, operational roles. Root level 0 tree is a view over separately typed governance, LINE, functional and delegation relationships.

**Priority C — Duty and Job Description:** Migrate/function charter with versioned accountable owner per business function/site/time; RACI/SoD conflict tests; position and person duty/JD versions. Do not attach confidential fields by default.

**Priority D — 24/7 coverage:** Establish continuous duty posts independent of employee workweek, staffing availability, relief, rest, night/holiday, handover and explicit uncovered intervals. 168h/week for one continuous staffed post; 40–42h/worker is a company target and separate policy constraint, not permission to overwork.

## P1 — Operationalization

- Unified HR navigation and module metadata, user task tests for HR, payroll, manager, employee and Shift Leader.
- Versioned payroll tax/BHXH rules, error reconciliation, employer/tenant-specific access, OIDC/MFA/service identity and privacy review.
- Code release governance: semver/tag; complete Windows-native Win10 service/tray UAT, signed releases, migration ledger, offsite backup and recovery, metrics/SLO and support runbook.
- Formal AppGate `build/high` evaluation: SSOT `plantops/app-gate/framework/enterprise-v2026.11.2.yaml`, 41 UAT-applicable criteria, 20 hard gates (OT mode increases applicability), score >=80/100 + 100% coverage + every evidence-backed hard gate + independent approver. No AppGate scoring/signoff is asserted here.

## Non-goals in this v0.11 changeset

CI v0.11 at this commit is NOT verified: GitHub Actions jobs end almost immediately without executable steps/logs; failure cause not established. No SQLite business schema migrations, import of actual PII, hourly payroll engine, new approval workflow, roster solver, native device integration, or independent UAT. No production readiness claims. Existing synthetic data remains unchanged.

## KB intake addendum: onboarding, identity, 2FA for mixed workforce (2026-11-19)

New knowledge-base source: [`docs/kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md`](kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md), preserved unchanged. Security adaptation and implementation backlog: [`docs/kb/ONBOARDING-IDENTITY-2FA-ORGAXIS-INTEGRATION.md`](kb/ONBOARDING-IDENTITY-2FA-ORGAXIS-INTEGRATION.md).

Mapping: `SEC-01` identity/MFA enrollment and recovery; `SEC-02` actor/subject audit; `PERSON-02` employee onboarding; `FLOW-01` supervised delegation; `UX-01` non-email/smartphone kiosk flows; `PLAT-02` SQLite migration; AppGate SEC/DAT/GOV security gates. The KB contains proposals, not executable features; prior maturity scores and mean **2.14/5** remain unchanged. DOB-based default password and static QR as sole sign-in method are **not approved**.
