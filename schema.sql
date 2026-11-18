PRAGMA foreign_keys = ON;

-- All identifiers and evidence in this sample are fabricated. No real PII.
-- Dates are ISO 8601; [valid_from, valid_to) ranges, NULL valid_to = open.
CREATE TABLE tenant (
  id TEXT PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL
);
CREATE TABLE feature_setting (
  tenant_id TEXT NOT NULL REFERENCES tenant(id), module TEXT NOT NULL,
  mode TEXT NOT NULL CHECK(mode IN ('core','off','record_only','enforce')),
  PRIMARY KEY (tenant_id,module)
);
CREATE TABLE legal_entity (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL, name TEXT NOT NULL, jurisdiction TEXT NOT NULL DEFAULT 'VN',
  UNIQUE(tenant_id,code)
);
CREATE TABLE corporate_relationship (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  parent_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  child_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  relation_type TEXT NOT NULL, valid_from TEXT NOT NULL, valid_to TEXT,
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE change_set (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  domain TEXT NOT NULL, title TEXT NOT NULL,
  effective_date TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('draft','approved','published','cancelled')),
  scenario TEXT NOT NULL DEFAULT 'official',
  recorded_at TEXT NOT NULL, source_ref TEXT
);
CREATE TABLE change_item (
  id TEXT PRIMARY KEY, change_set_id TEXT NOT NULL REFERENCES change_set(id),
  entity_name TEXT NOT NULL, entity_key TEXT NOT NULL,
  operation TEXT NOT NULL, details TEXT NOT NULL
);
CREATE TABLE org_unit (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  legal_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  code TEXT NOT NULL, name TEXT NOT NULL, unit_type TEXT NOT NULL,
  created_at TEXT NOT NULL, UNIQUE(tenant_id,code)
);
CREATE TABLE org_unit_relation (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  parent_id TEXT NOT NULL REFERENCES org_unit(id),
  child_id TEXT NOT NULL REFERENCES org_unit(id),
  relation_type TEXT NOT NULL DEFAULT 'LINE',
  valid_from TEXT NOT NULL, valid_to TEXT,
  change_set_id TEXT REFERENCES change_set(id),
  CHECK(parent_id<>child_id), CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE job_profile (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  name TEXT NOT NULL, family TEXT NOT NULL
);
CREATE TABLE position (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  job_profile_id TEXT NOT NULL REFERENCES job_profile(id),
  code TEXT NOT NULL, title TEXT NOT NULL, holder_limit INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL, UNIQUE(tenant_id,code), CHECK(holder_limit>0)
);
CREATE TABLE position_placement (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  position_id TEXT NOT NULL REFERENCES position(id),
  org_unit_id TEXT NOT NULL REFERENCES org_unit(id),
  valid_from TEXT NOT NULL, valid_to TEXT,
  change_set_id TEXT REFERENCES change_set(id),
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE position_reporting (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  position_id TEXT NOT NULL REFERENCES position(id),
  supervisor_position_id TEXT NOT NULL REFERENCES position(id),
  relation_type TEXT NOT NULL DEFAULT 'LINE',
  valid_from TEXT NOT NULL, valid_to TEXT,
  change_set_id TEXT REFERENCES change_set(id),
  CHECK(position_id<>supervisor_position_id),
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE staffing_authorization (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  position_id TEXT NOT NULL REFERENCES position(id), approved_fte REAL NOT NULL,
  valid_from TEXT NOT NULL, valid_to TEXT, source_ref TEXT,
  CHECK(approved_fte>=0), CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE person (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  display_name TEXT NOT NULL, nationality TEXT NOT NULL DEFAULT 'VN',
  synthetic INTEGER NOT NULL DEFAULT 1 CHECK(synthetic=1)
);
CREATE TABLE employment_episode (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), legal_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  relation_type TEXT NOT NULL DEFAULT 'employee',
  valid_from TEXT NOT NULL, valid_to TEXT, source_ref TEXT,
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE assignment (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  employment_id TEXT NOT NULL REFERENCES employment_episode(id),
  position_id TEXT NOT NULL REFERENCES position(id),
  assignment_type TEXT NOT NULL CHECK(assignment_type IN ('primary','acting','secondary','secondment')),
  fte REAL NOT NULL DEFAULT 1.0, valid_from TEXT NOT NULL, valid_to TEXT,
  change_set_id TEXT REFERENCES change_set(id),
  CHECK(fte>=0 AND fte<=1), CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE hr_event (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  event_type TEXT NOT NULL, effective_date TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'published',
  change_set_id TEXT REFERENCES change_set(id), details TEXT NOT NULL
);
CREATE TABLE evidence (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT REFERENCES person(id),
  category TEXT NOT NULL, source_type TEXT NOT NULL,
  verification_status TEXT NOT NULL CHECK(verification_status IN ('SELF_REPORTED','DOCUMENTED','VERIFIED','DISPUTED','UNKNOWN')),
  source_ref TEXT NOT NULL, recorded_at TEXT NOT NULL
);
CREATE TABLE external_work_history (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), employer_label TEXT NOT NULL,
  occupation_label TEXT NOT NULL, valid_from TEXT NOT NULL, valid_to TEXT,
  evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE occupation_period (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  employer_label TEXT NOT NULL, occupation_label TEXT NOT NULL,
  condition_category TEXT CHECK(condition_category IN ('I','II','III','IV','V','VI','UNKNOWN')),
  valid_from TEXT NOT NULL, valid_to TEXT,
  evidence_id TEXT REFERENCES evidence(id),
  classification_rule_ref TEXT,
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE skill (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL, label TEXT NOT NULL, UNIQUE(tenant_id,code)
);
CREATE TABLE person_skill (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), skill_id TEXT NOT NULL REFERENCES skill(id),
  level INTEGER NOT NULL CHECK(level BETWEEN 1 AND 4),
  assessed_at TEXT NOT NULL, valid_until TEXT, evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE training_program (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  name TEXT NOT NULL, regulatory_basis TEXT, version TEXT NOT NULL
);
CREATE TABLE training_record (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  program_id TEXT NOT NULL REFERENCES training_program(id),
  completed_on TEXT NOT NULL, result TEXT NOT NULL CHECK(result IN ('passed','failed','attended')),
  valid_until TEXT, evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE qualification (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  qualification_type TEXT NOT NULL, issue_date TEXT, expiry_date TEXT,
  status TEXT NOT NULL, evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE work_task (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL, name TEXT NOT NULL, UNIQUE(tenant_id,code)
);
CREATE TABLE task_requirement (
  task_id TEXT NOT NULL REFERENCES work_task(id),
  skill_id TEXT NOT NULL REFERENCES skill(id), min_level INTEGER NOT NULL,
  PRIMARY KEY(task_id, skill_id)
);
CREATE TABLE work_assignment (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), task_id TEXT NOT NULL REFERENCES work_task(id),
  start_at TEXT NOT NULL, end_at TEXT NOT NULL, location_code TEXT,
  CHECK(end_at>start_at)
);
CREATE TABLE kpi_result (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), period TEXT NOT NULL,
  metric_code TEXT NOT NULL, actual REAL, target REAL, source_type TEXT NOT NULL
);
CREATE TABLE recognition_case (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), case_type TEXT NOT NULL,
  effective_date TEXT, status TEXT NOT NULL, evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE discipline_case (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  opened_on TEXT NOT NULL, case_status TEXT NOT NULL,
  decision_type TEXT, effective_date TEXT, evidence_id TEXT REFERENCES evidence(id)
);

-- Compensation and payroll data; this demo does not calculate statutory rates.
CREATE TABLE compensation_agreement (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  employment_id TEXT NOT NULL REFERENCES employment_episode(id),
  currency TEXT NOT NULL, base_pay_minor INTEGER NOT NULL,
  valid_from TEXT NOT NULL, valid_to TEXT,
  source_ref TEXT NOT NULL, CHECK(base_pay_minor>=0),
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
CREATE TABLE pay_component (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL, name TEXT NOT NULL, component_type TEXT NOT NULL,
  UNIQUE(tenant_id,code)
);
CREATE TABLE payroll_run (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  legal_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  pay_period TEXT NOT NULL, status TEXT NOT NULL,
  rule_version TEXT, UNIQUE(tenant_id,legal_entity_id,pay_period)
);
CREATE TABLE payroll_item (
  id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES payroll_run(id),
  employment_id TEXT NOT NULL REFERENCES employment_episode(id),
  component_id TEXT NOT NULL REFERENCES pay_component(id),
  amount_minor INTEGER NOT NULL, currency TEXT NOT NULL,
  source_ref TEXT NOT NULL
);

-- Absence + statutory benefits: leave decisions and claims are distinct.
CREATE TABLE absence_case (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  employment_id TEXT NOT NULL REFERENCES employment_episode(id),
  absence_type TEXT NOT NULL, valid_from TEXT NOT NULL, valid_to TEXT NOT NULL,
  approval_status TEXT NOT NULL, source_ref TEXT NOT NULL,
  CHECK(valid_to>valid_from)
);
CREATE TABLE social_insurance_period (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  legal_entity_id TEXT REFERENCES legal_entity(id),
  valid_from TEXT NOT NULL, valid_to TEXT,
  status TEXT NOT NULL, evidence_id TEXT REFERENCES evidence(id)
);
CREATE TABLE benefit_claim (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  absence_id TEXT REFERENCES absence_case(id),
  benefit_type TEXT NOT NULL, application_date TEXT NOT NULL,
  status TEXT NOT NULL, approved_amount_minor INTEGER,
  rule_ref TEXT, evidence_id TEXT REFERENCES evidence(id)
);

-- Tax determination is case-based, jurisdictional, time-dependent.
CREATE TABLE tax_profile (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  jurisdiction TEXT NOT NULL, residency_status TEXT NOT NULL,
  dependent_count INTEGER NOT NULL DEFAULT 0,
  valid_from TEXT NOT NULL, valid_to TEXT,
  CHECK(dependent_count>=0)
);
CREATE TABLE tax_withholding (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  payroll_run_id TEXT NOT NULL REFERENCES payroll_run(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  amount_minor INTEGER NOT NULL, currency TEXT NOT NULL, source_ref TEXT NOT NULL
);

-- Immigration is NOT the same as legal right to work.
CREATE TABLE travel_document (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  document_type TEXT NOT NULL, masked_reference TEXT NOT NULL,
  issuing_country TEXT NOT NULL, expiry_date TEXT NOT NULL,
  status TEXT NOT NULL
);
CREATE TABLE immigration_credential (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id), sponsor_entity_id TEXT REFERENCES legal_entity(id),
  credential_type TEXT NOT NULL, reference_masked TEXT NOT NULL,
  valid_from TEXT NOT NULL, valid_to TEXT NOT NULL,
  status TEXT NOT NULL, CHECK(valid_to>valid_from)
);
CREATE TABLE entry_exit_event (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  event_type TEXT NOT NULL CHECK(event_type IN ('ENTRY','EXIT')),
  occurred_at TEXT NOT NULL, source_type TEXT NOT NULL
);
CREATE TABLE work_authorization (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
  person_id TEXT NOT NULL REFERENCES person(id),
  legal_entity_id TEXT NOT NULL REFERENCES legal_entity(id),
  authorization_type TEXT NOT NULL CHECK(authorization_type IN ('PERMIT','EXEMPTION_CONFIRMATION','EXEMPTION_NOTIFICATION')),
  status TEXT NOT NULL, role_scope TEXT NOT NULL, worksite_scope TEXT,
  valid_from TEXT NOT NULL, valid_to TEXT NOT NULL,
  rule_ref TEXT, evidence_id TEXT REFERENCES evidence(id),
  CHECK(valid_to>valid_from)
);

CREATE TABLE audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id TEXT NOT NULL REFERENCES tenant(id),
  happened_at TEXT NOT NULL, actor TEXT NOT NULL,
  event_name TEXT NOT NULL, target TEXT NOT NULL, payload TEXT NOT NULL
);
CREATE INDEX ix_org_rel_dates ON org_unit_relation(tenant_id,child_id,valid_from,valid_to);
CREATE INDEX ix_reporting_dates ON position_reporting(tenant_id,position_id,valid_from,valid_to);
CREATE INDEX ix_assign_dates ON assignment(tenant_id,person_id,valid_from,valid_to);
CREATE INDEX ix_employment_dates ON employment_episode(tenant_id,person_id,valid_from,valid_to);
CREATE INDEX ix_hr_events ON hr_event(tenant_id,effective_date,event_type);
CREATE INDEX ix_occupational ON occupation_period(tenant_id,person_id,valid_from,valid_to);
CREATE INDEX ix_payroll ON payroll_item(run_id,employment_id);
CREATE INDEX ix_work_auth ON work_authorization(tenant_id,person_id,valid_from,valid_to);
