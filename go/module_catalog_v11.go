package main

import (
  "net/http"
  "strings"
)

// OrgAxis v0.11: source-evidenced pilot inventory, not AppGate acceptance.
// Maturity: 0 absent, 1 design, 2 partial demo, 3 synthetic tested,
// 4 real data integration/UAT, 5 independently accepted production.
// Scores include halves for partial engineering work. All entries must be
// manually reconciled with code and evidence before raising maturity.
type moduleV11 struct {
  ID string `json:"id"`
  Group string `json:"group"`
  Name string `json:"name"`
  Maturity float64 `json:"maturity"`
  Status string `json:"status"`
  Scope string `json:"scope"`
  Functions []string `json:"functions"`
  Evidence []string `json:"evidence"`
  Gaps []string `json:"gaps"`
}

var modulesV11 = []moduleV11{
  {ID:"ORG-01",Group:"ORG",Name:"Pháp nhân và tenant",Maturity:2,Status:"partial_demo",Scope:"Khai báo pháp nhân, metadata tenant; phạm vi demo một tenant",Functions:[]string{"Danh sách/tạo công ty", "tenant profile", "corporate relationship"},Evidence:[]string{"go/main.go", "schema.sql"},Gaps:[]string{"Cách ly tenant thực", "legal-entity operations"}},
  {ID:"ORG-02",Group:"ORG",Name:"Cấu trúc đơn vị theo thời gian",Maturity:3,Status:"operational_demo",Scope:"Blank organization, unit tree, as-of, chuyển Warehouse mẫu",Functions:[]string{"Đọc cây đơn vị", "thay đổi cha-con theo ngày", "lịch sử"},Evidence:[]string{"go/main.go", "go/main_test.go", "seed.py"},Gaps:[]string{"Tạo/đóng/gộp/tách và approval workflow"}},
  {ID:"ORG-03",Group:"ORG",Name:"Vị trí, tuyến báo cáo, biên chế",Maturity:3,Status:"operational_demo",Scope:"Position placement, reporting, holder limit, manager chain, đổi tuyến",Functions:[]string{"Tra manager", "chain", "direct reports", "move", "cycle guard"},Evidence:[]string{"go/main.go", "go/main_test.go", "go/security_v06_test.go"},Gaps:[]string{"Position slots độc lập", "acting/dual reporting", "vacancy"}},
  {ID:"ORG-04",Group:"ORG",Name:"Change-set tái cơ cấu",Maturity:2,Status:"partial_demo",Scope:"Có change_set và event mô phỏng, chưa đủ vòng đời phê duyệt",Functions:[]string{"Ghi nhận change", "tra lịch sử", "backdated read"},Evidence:[]string{"schema.sql", "seed.py", "go/main.go"},Gaps:[]string{"Draft/review/approve/publish", "prospective merger/split"}},
  {ID:"ORG-05",Group:"ORG",Name:"Function Charter & RACI",Maturity:1,Status:"design_only",Scope:"Đặc tả đầu mối A duy nhất theo chức năng và phạm vi",Functions:[]string{"Chức năng khối/phòng", "RACI", "SoD", "conflict detection"},Evidence:[]string{"docs/ORG-DUTY-COVERAGE-24x7-SPEC.md"},Gaps:[]string{"Chưa có bảng, API, validation, UI"}},
  {ID:"ORG-06",Group:"ORG",Name:"Job Description & duty từng người",Maturity:1,Status:"design_only",Scope:"Đặc tả job family, vị trí, nhiệm vụ kiêm nhiệm và thay thế",Functions:[]string{"JD version", "nhiệm vụ vị trí", "nhiệm vụ cá nhân", "deputy/acting"},Evidence:[]string{"docs/ORG-DUTY-COVERAGE-24x7-SPEC.md"},Gaps:[]string{"Chưa có migration, quyền cập nhật, lịch sử nhiệm vụ"}},
  {ID:"PERSON-01",Group:"PERSON",Name:"Hồ sơ và lịch sử công việc",Maturity:3,Status:"operational_demo",Scope:"Person, employment, position history; API hồ sơ phân quyền",Functions:[]string{"Tìm kiếm", "xem dossier", "employment/assignment timeline"},Evidence:[]string{"go/modules.go", "go/workflow_v06.go", "go/security_v06_test.go"},Gaps:[]string{"Hồ sơ pháp lý và PII thật", "import source of record"}},
  {ID:"PERSON-02",Group:"PERSON",Name:"Hire/transfer/separation",Maturity:2.5,Status:"partial_demo",Scope:"Có hire demo, chuyển vị trí, kết thúc việc theo ngày",Functions:[]string{"Hire", "move", "separation", "HR events"},Evidence:[]string{"go/workforce.go", "go/main.go", "go/modules_test.go"},Gaps:[]string{"Hợp đồng", "rehire", "pháp nhân khác", "phê duyệt thực"}},
  {ID:"PERSON-03",Group:"PERSON",Name:"Lịch sử nghề và điều kiện lao động",Maturity:2,Status:"partial_demo",Scope:"Có occupation periods và nhóm điều kiện synthetic",Functions:[]string{"Tra thời gian nghề", "lịch sử tiếp xúc/điều kiện", "source ref"},Evidence:[]string{"schema.sql", "go/modules.go"},Gaps:[]string{"Thẩm định quy tắc độc hại, phụ cấp, hồ sơ xác minh"}},
  {ID:"PERSON-04",Group:"PERSON",Name:"Chuyên gia nước ngoài",Maturity:2,Status:"partial_demo",Scope:"Masked travel/immigration/authorization demo",Functions:[]string{"Tra giấy tờ", "quyền làm việc theo record", "hạn 90 ngày"},Evidence:[]string{"go/enterprise_modules.go", "go/modules_test.go"},Gaps:[]string{"Workflow pháp lý, cảnh báo, nguồn chứng từ"}},
  {ID:"TALENT-01",Group:"TALENT",Name:"Kỹ năng và điều kiện giao việc",Maturity:2.5,Status:"partial_demo",Scope:"Đánh giá kỹ năng L1–L4, kiểm tra điều kiện kỹ năng, không cấp phép vận hành",Functions:[]string{"Danh mục", "assessment", "holder search", "skill-only check"},Evidence:[]string{"go/modules.go", "go/workforce.go", "go/modules_test.go"},Gaps:[]string{"Kỹ năng theo thiết bị", "giấy phép", "hạn chứng chỉ", "HSE gates"}},
  {ID:"TALENT-02",Group:"TALENT",Name:"Đào tạo, chứng chỉ",Maturity:2.5,Status:"partial_demo",Scope:"Training catalog, record, thời hạn hiệu lực; chứng chỉ chưa được cấp tự động",Functions:[]string{"Chương trình", "kết quả", "học viên", "ngày hết hạn"},Evidence:[]string{"go/modules.go", "go/modules_test.go"},Gaps:[]string{"Kế hoạch đào tạo", "bắt buộc theo vị trí", "hồ sơ phê duyệt"}},
  {ID:"TALENT-03",Group:"TALENT",Name:"KPI và đánh giá hiệu suất",Maturity:2,Status:"partial_demo",Scope:"Lưu và đọc KPI result synthetic",Functions:[]string{"KPI theo kỳ", "mục tiêu/thực hiện", "nhập kết quả"},Evidence:[]string{"go/modules.go", "go/modules_test.go"},Gaps:[]string{"Mẫu đánh giá", "người duyệt", "liên kết nguồn nghiệp vụ"}},
  {ID:"TALENT-04",Group:"TALENT",Name:"Khen thưởng",Maturity:2.5,Status:"partial_demo",Scope:"Đề xuất, review HR, phê duyệt Director; demo Kaizen tiers",Functions:[]string{"Mở proposal", "tính bậc demo", "tuyến duyệt", "lịch sử"},Evidence:[]string{"go/workflow_v06.go", "go/security_v06_test.go"},Gaps:[]string{"Quy chế thưởng thực", "chứng từ", "hạch toán"}},
  {ID:"TALENT-05",Group:"TALENT",Name:"Kỷ luật và hồ sơ vụ việc",Maturity:2,Status:"partial_demo",Scope:"Luồng trình duyệt sự việc; không tự ra quyết định kỷ luật pháp lý",Functions:[]string{"Mở case", "cấp xét duyệt", "audit"},Evidence:[]string{"go/workflow_v06.go", "go/modules.go"},Gaps:[]string{"SoD/Legal, hồ sơ, privacy và luật lao động"}},
  {ID:"FLOW-01",Group:"FLOW",Name:"Quy trình phê duyệt theo tuyến",Maturity:3,Status:"operational_demo",Scope:"LINE_MANAGER theo as-of, escalation leave, role-based case approval",Functions:[]string{"Tuyến 1–n", "case create/decision/resubmit", "no self-approval"},Evidence:[]string{"go/workflow_v06.go", "go/security_v06_test.go"},Gaps:[]string{"Deputy/acting/delegation/functional routing", "SoD phức tạp"}},
  {ID:"FLOW-02",Group:"FLOW",Name:"Nghỉ phép và vắng mặt",Maturity:2.5,Status:"partial_demo",Scope:"Có case, phê duyệt và demo counting; chưa gắn đầy đủ vào time close",Functions:[]string{"Xin nghỉ", "xét duyệt", "lịch sử", "leave credit model"},Evidence:[]string{"go/modules.go", "go/workflow_v06.go", "go/attendance_close_v09.go"},Gaps:[]string{"Quota/accrual, lễ, ca, entitlement, evidence binding"}},
  {ID:"WFM-01",Group:"WFM",Name:"Mẫu ca và roster",Maturity:2.5,Status:"partial_demo",Scope:"6.000 ca synthetic, overnight và mẫu SHIFT/REST/HOLIDAY",Functions:[]string{"Tra roster", "day plan per person/month", "shift match"},Evidence:[]string{"go/attendance_v07.go", "go/attendance_close_v09.go", "go/attendance_seed_v07.go"},Gaps:[]string{"Rotation, đổi ca, 40–42h policy, rest và holidays được xác nhận"}},
  {ID:"WFM-02",Group:"WFM",Name:"Thiết bị và raw punches",Maturity:3,Status:"operational_demo",Scope:"Import idempotent/quarantine; simulated JSON pull; checkpoint giao dịch",Functions:[]string{"Device/badge binding", "pull page", "exception", "append-only triggers"},Evidence:[]string{"go/attendance_v07.go", "go/attendance_checkpoint_v08.go", "go/attendance_checkpoint_v08_test.go"},Gaps:[]string{"Máy thực", "event provenance", "auto-poll, offline 8h, gateway"}},
  {ID:"WFM-03",Group:"WFM",Name:"Tính công và exception",Maturity:3,Status:"operational_demo",Scope:"Pure-Go event pairing, ngày công và anomaly; synthetic unit tests",Functions:[]string{"IN/OUT", "ca", "duplicate", "missing", "OT/leave eval"},Evidence:[]string{"go/timecore/engine.go", "go/timecore/engine_test.go", "go/attendance_v07_test.go"},Gaps:[]string{"Golden attendance data", "site policy", "correction approval"}},
  {ID:"WFM-04",Group:"WFM",Name:"Khóa kỳ công",Maturity:2.5,Status:"partial_demo",Scope:"Plan 28–31 ngày, readiness blockers, HR prepare/PAYROLL lock, source digest",Functions:[]string{"Calendar import", "readiness", "prepare", "approve", "verify drift"},Evidence:[]string{"go/attendance_close_v09.go", "go/attendance_close_v09_test.go"},Gaps:[]string{"Batch close, correction/reopen, leave OT binding, real UAT"}},
  {ID:"WFM-05",Group:"WFM",Name:"OT và chứng cứ phê duyệt",Maturity:2,Status:"partial_demo",Scope:"Có model interval approved và tính thử; chưa chứng minh case linkage",Functions:[]string{"Approved OT time segment", "pay preview gating"},Evidence:[]string{"go/attendance_v07.go", "go/timecore/engine.go"},Gaps:[]string{"VN rule pack, consent, limits, approved case provenance"}},
  {ID:"WFM-06",Group:"WFM",Name:"Coverage 365/24/7 và bàn giao",Maturity:1,Status:"design_only",Scope:"Đặc tả coverage post, staffing gap, duty handover, relief FTE",Functions:[]string{"Coverage demand", "roster optimization", "gap report", "handover"},Evidence:[]string{"docs/ORG-DUTY-COVERAGE-24x7-SPEC.md"},Gaps:[]string{"Chưa có implementation, scheduler, heatmap"}},
  {ID:"PAY-01",Group:"PAY",Name:"Bảng lương demo và gross preview",Maturity:2,Status:"partial_demo",Scope:"12k payslips synthetic độc lập time engine; gross preview blocked khi thiếu nguồn",Functions:[]string{"Payroll year/person", "workbench", "payslip", "PIT simulation demo"},Evidence:[]string{"go/payroll_workbench.go", "go/workflow_v06.go", "go/attendance_v07.go"},Gaps:[]string{"Source-to-pay", "reconciliation", "approved month integration"}},
  {ID:"PAY-02",Group:"PAY",Name:"Lương pháp lý VN, PIT/BHXH",Maturity:0.5,Status:"not_implemented",Scope:"Chưa có engine tính lương, PIT, BHXH được HR/Finance đối soát",Functions:[]string{"Versioned rules dự kiến", "payslip thật", "export ngân hàng"},Evidence:[]string{"docs/HRM-BENCHMARK-2026.md", "go/README-v09.md"},Gaps:[]string{"Tất cả logic pháp lý, UAT và chi trả thực"}},
  {ID:"PAY-03",Group:"PAY",Name:"Phúc lợi và bảo hiểm hồ sơ",Maturity:1.5,Status:"partial_demo",Scope:"Benefit claim registry, SI periods, tax profiles ở mức record",Functions:[]string{"Claim create/list", "coverage status", "tax profile record"},Evidence:[]string{"schema.sql", "go/enterprise_modules.go"},Gaps:[]string{"Entitlement engine, tính đóng/hưởng, quyết toán"}},
  {ID:"SEC-01",Group:"SEC",Name:"Định danh, tài khoản, RBAC",Maturity:3,Status:"operational_demo",Scope:"Local personal login, session, CSRF, roles, scope test",Functions:[]string{"Login/logout", "user create", "self/manager payroll read"},Evidence:[]string{"go/security_v06.go", "go/security_v06_test.go"},Gaps:[]string{"SSO/OIDC MFA, service accounts, legal employer scopes"}},
  {ID:"SEC-02",Group:"SEC",Name:"Audit và riêng tư",Maturity:2,Status:"partial_demo",Scope:"Actor audit cho workflow; source digest/trigger raw; chưa đủ PII governance",Functions:[]string{"Audit events", "role-scoped views", "ledger trace"},Evidence:[]string{"go/security_v06.go", "go/attendance_close_v09.go", "go/workflow_v06.go"},Gaps:[]string{"Privacy inventory, tamper-evidence, retention, export/delete"}},
  {ID:"INT-01",Group:"INT",Name:"Adapter máy công / ERP",Maturity:1.5,Status:"partial_demo",Scope:"HTTP JSON contract máy công, chưa có ERP live hoặc vendor gateway",Functions:[]string{"Vendor-neutral fetch", "cursor", "error sanitization"},Evidence:[]string{"go/timecore/adapter.go", "go/attendance_checkpoint_v08.go"},Gaps:[]string{"Thiết bị và ERP thực", "OpenAPI", "reconciliation"}},
  {ID:"INT-02",Group:"INT",Name:"Import HR/Org khuyết danh",Maturity:0.5,Status:"not_implemented",Scope:"Seed Python, chưa có pipeline CSV import review/approve/merge",Functions:[]string{"Import structure, slots, edges, function charter dự kiến"},Evidence:[]string{"seed.py", "docs/ORG-DUTY-COVERAGE-24x7-SPEC.md"},Gaps:[]string{"CSV validation, preview, dry-run, approval, rollback"}},
  {ID:"PLAT-01",Group:"PLAT",Name:"Windows service/tray và artifact",Maturity:3,Status:"operational_demo",Scope:"Go cross-build Win10 x64, tray/service scripts, singleton test",Functions:[]string{"GUI/server EXE", "installer", "launch", "single-writer lock"},Evidence:[]string{"go/runtime.go", "go/windows/Install-OrgAxis-Service.ps1", "go/ops_test.go"},Gaps:[]string{"UAT Windows thật", "service recovery", "code signing"}},
  {ID:"PLAT-02",Group:"PLAT",Name:"SQLite migrations/backup/restore",Maturity:2.5,Status:"partial_demo",Scope:"Additive upgrades và VACUUM INTO pre-upgrade snapshots",Functions:[]string{"v06–v09 schema migrations", "snapshots", "locked rows"},Evidence:[]string{"go/runtime.go", "go/attendance_backup_v07.go", "go/attendance_close_v09.go"},Gaps:[]string{"Migration ledger, restore drill, off-host backups"}},
  {ID:"PLAT-03",Group:"PLAT",Name:"Logs/health/diagnostics",Maturity:2,Status:"partial_demo",Scope:"Rotating log, status, health, limited LAN TLS read-only",Functions:[]string{"Health", "bounded logs", "uptime", "LAN bearer"},Evidence:[]string{"go/ops.go", "go/ops_test.go", "go/main.go"},Gaps:[]string{"Alerts, metrics, SLO, incident runbook"}},
  {ID:"UX-01",Group:"UX",Name:"Workspaces và overview-to-detail",Maturity:3,Status:"operational_demo",Scope:"Org explorer 1k nodes, person detail, leave/bonus/discipline, time lab; nhiều UI cũ",Functions:[]string{"Workspace", "org as-of", "person pane", "time and payroll demo"},Evidence:[]string{"go/assets/workspace.js", "go/assets/app.js", "go/assets/time.js"},Gaps:[]string{"Task UAT, module navigation unify, WCAG/device"}},
  {ID:"GATE-01",Group:"GATE",Name:"AppGate evidence và UAT",Maturity:1,Status:"design_only",Scope:"AppGate intake và evidence map, chưa có independent score",Functions:[]string{"AppGate rubric mapping", "PR docs", "CI logs"},Evidence:[]string{"docs/APPGATE-HR-READINESS.md", "plantops/app-gate/framework/enterprise-v2026.11.2.yaml"},Gaps:[]string{"Actual signed assessment, evidence coverage, all hard gates"}},
}

var moduleGroupsV11 = map[string]string{
  "ORG":"Cơ cấu & chức năng", "PERSON":"Hồ sơ & vòng đời nhân sự",
  "TALENT":"Năng lực & nhân tài", "FLOW":"Workflow & nghỉ phép",
  "WFM":"Ca, công & coverage", "PAY":"Lương & phúc lợi",
  "SEC":"Định danh, an toàn & audit", "INT":"Tích hợp & dữ liệu",
  "PLAT":"Nền tảng & vận hành", "UX":"Trải nghiệm", "GATE":"AppGate",
}

func moduleInventoryV11(r *http.Request) (any,error) {
  group:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("group")))
  status:=strings.TrimSpace(r.URL.Query().Get("status"))
  if group!="" {
    if _,ok:=moduleGroupsV11[group];!ok{return nil,fail(422,"Unknown catalog group")}
  }
  if status!="" && status!="operational_demo" && status!="partial_demo" && status!="design_only" && status!="not_implemented" {
    return nil,fail(422,"Unknown module status")
  }
  items:=make([]moduleV11,0,len(modulesV11))
  countByStatus:=map[string]int{}
  total:=0.0
  for _,m:=range modulesV11 {
    if group!="" && m.Group!=group {continue}
    if status!="" && m.Status!=status {continue}
    items=append(items,m)
    countByStatus[m.Status]++
    total+=m.Maturity
  }
  avg:=0.0
  if len(items)>0 {avg=float64(int(total/float64(len(items))*100+0.5))/100}
  return map[string]any{
    "version":"0.11", "catalog_type":"source_review", "data_class":"synthetic_pilot",
    "appgate_assessment":"NOT_ASSESSED", "baseline":"org-duties-coverage-24x7-v10@475b2c9",
    "scoring":"engineering_maturity_not_appgate", "count":len(items),
    "average_maturity":avg,"status_counts":countByStatus, "groups":moduleGroupsV11,"modules":items,
    "warning":"Scores are source-based engineering assessments; not field UAT or go-live approval.",
  },nil
}
