# OrgAxis HRM Competitive Benchmark — 18 November 2026

**Status:** Source-based capability benchmark, **NOT** product hands-on testing, procurement ranking or certification. All current OrgAxis figures are from inspected branch `feature/hr-workspace-rbac-v06`, except v0.7 prototype updates explicitly identified.

## Why this benchmark exists

OrgAxis serves ~1,000 synthetic staff in a Vietnamese cement-factory scenario. The differentiation is temporal legal structure, vacancy/position/reporting, dated workflow authority, multiple companies and industrial capability/compliance. **The blocking gap for HR use is the time–attendance–leave–overtime–payroll chain, not a lack of UI screens.** A synthetic 12,000-row payroll ledger that cannot be traced to rosters, events and approved exceptions is demonstration data, not automated payroll.

## Vietnamese systems — public capabilities only

| Reference | Verified from published supplier material | OrgAxis takeaway |
|---|---|---|
| MISA AMIS HRM | Device connection, shift definitions, attendance detail/summary, leave and OT requests, locking attendance and transfer to payroll | Adopt lifecycle `raw -> day -> exception review -> lock -> payroll`, device identity reconciliation, VN leave policy |
| Base HRM+ (Checkin, Schedule, Payroll) | Checkin sources, flexible shift schedule, payroll integration and employee payslips | Keep separate modules but canonical IDs, one approved time ledger |
| 1Office HRM | Multi-device attendance, leave/OT forms, HR records, calculation of wages and payslips | Employee/supervisor self-service with complete monthly timecard |
| Tanca | Biometric/time clock and multi-location employee attendance, manufacturing use | Treat physical terminals, sites and outage handling as first-class devices |

These are **not assertions of data accuracy, independent security certification, supported model lists or prices**.

Sources (vendor):
- MISA: https://helpamis.misa.vn/amis-cham-cong/kb/huong-dan-luong-nghiep-vu-tren-amis-cham-cong/
- MISA detailed timecards: https://helpamis.misa.vn/amis-cham-cong/kb/bang-cham-cong-chi-tiet-lam-them-gio/
- MISA devices: https://helpamis.misa.vn/amis-cham-cong/kb/loai-may-cham-cong-nao-ket-noi-tot-nhat-voi-phan-mem/
- Base: https://base.vn/platform/hrm ; https://base.vn/app/schedule
- 1Office: https://1office.vn/cong-ca-luong ; https://1office.vn/cham-cong
- Tanca: https://h.tanca.vn/vi/industries/manufacturing

## International systems — architectural patterns

| Reference | Supplier-documented capability | OrgAxis takeaway |
|---|---|---|
| SAP SuccessFactors Time Tracking | External time events and device events, event pairing, valuation, time sheet and payroll integration | Separate *punch evidence*, *time event pairing*, *time valuation* and payroll posting |
| Workday | Calculated time blocks, schedules, absence, manager actions and payroll integration | Schedule and absence affect payable regular time; allow independent review of calculated time blocks |
| Oracle Fusion Time and Labor | Time collection device imports, badge/time exceptions, rule validations, approval and payroll transfer | Build ingest quarantine and reprocessing; do not assume unknown badge is a valid worker |
| UKG Pro WFM | Time clocks, scheduling, pay rules, absence and operational exception monitoring | Exception dashboard and supervisor timecard review should precede pay period close |

Sources (supplier):
- SAP: https://help.sap.com/docs/successfactors-employee-central/operating-time-management-in-sap-successfactors/9b874bb10627450fad5c2ceff72c107f.html
- Workday: https://doc.workday.com/admin-guide/en-us/workday-feature-descriptions/workday-workforce-management/future--time-tracking.html
- Oracle: https://docs.oracle.com/en/cloud/saas/human-resources/faitl/overview-of-time-collection-device-and-web-clock-event.html
- UKG: https://www.ukg.com/products/ukg-pro

## Capability gap — evidence-based, not scored by vendor name

| Product capability | OrgAxis v0.6 evidence | v0.7 prototype | Acceptance to claim parity on this slice |
|---|---|---|---|
| Person, employment | 1,000 synthetic profiles / dated assignment | Reuses same IDs | Contract/entry/exit/rehire integrity and import reconciliation |
| Org reporting | As-of person chain and 1,000-node bird's eye | Reuses current authority | Supervisor/acting delegation, cycle, vacancy, full audit and dated workflow routing |
| Biometric import | Missing | Normalized JSON pull contract, HTTP intake, idempotence and quarantine | Real device/model validated; IT/OT isolation and retry test |
| Shift schedule | Missing | 6,000 synthetic shift rows, dates with midnight crossing | Work pattern, holidays, rotation, shift changes, union-agreed rest rules |
| Calculated attendance | Missing | Pure-Go evaluation, exceptions, paid-leave and approved OT tests | HR-verified daily/monthly ledger for 2 real payroll periods |
| Leave | Demo decision workflow, simplified Mon–Sat count | Paid leave credit contract | Entitlement/accrual, holiday calendar, shift-aware leave, carried forward, approval linkage |
| Overtime | Not linked to punches | Approved interval gated; unapproved outside time blocks close | Consent, tiered approval, maximum hours, night supplements and compensatory rest |
| Payroll | 12,000 SYNTHETIC ledger rows, no time derivation | Gross preview sourced from shift and approved OT; incomplete periods BLOCK | Full wage-component provenance; BHXH/PIT validated; retro, reconciliation and close |
| Access/security | Role/scope/session/CSRF local demo | Device import limited to HR/ADMIN; payroll read scoped | OIDC+MFA, service accounts, tenant boundary, security assessment |

## Vietnamese labor-rule boundary

Rules for ordinary/weekend/holiday overtime, night work, overtime consent and limits depend on law and employment agreements. **Do not hard-code employee wage entitlement or automatically assert statutory compliance from one time-clock event.** Production calculation needs effective-dated legal packs and a legal/HR signoff. Official background: https://congbao.chinhphu.vn/van-ban/van-ban-hop-nhat-so-18-vbhn-vpqh-468971/62878.htm and https://baochinhphu.vn/can-cu-tinh-tien-luong-lam-them-gio-102261117150954364.htm

## Competitive positioning

**Do not market full HRM parity.** Pitch OrgAxis as a temporal organization + industrial workforce engine with a developing work-time/payroll chain. The demo must prove complete source-to-pay traceability before HR signs off.
