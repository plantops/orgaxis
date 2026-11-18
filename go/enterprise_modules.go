package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Make a consistent SQLite snapshot before upgrading an existing demo database.
// This uses SQLite's VACUUM INTO rather than copying a potentially WAL-backed file.
func backupBeforeUpgrade(db SQLiteDB, dbPath string) (string, error) {
	row, err := first(db, `SELECT COUNT(*) AS n FROM person WHERE tenant_id=? AND synthetic=1`, tenantID)
	if err != nil {
		return "", err
	}
	if intval(row["n"]) < 1000 {
		return "", nil
	}
	existing, err := first(db, `SELECT COUNT(*) AS n FROM skill WHERE tenant_id=?`, tenantID)
	if err != nil {
		return "", err
	}
	if intval(existing["n"]) > 0 {
		return "", nil
	}
	tag := time.Now().Format("20060102-150405")
	path := filepath.Join(filepath.Dir(dbPath), fmt.Sprintf("orgaxis-before-module-upgrade-%s.sqlite3", tag))
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("migration backup already exists: %s", path)
	}
	if err = db.Exec(`VACUUM INTO ?`, path); err != nil {
		// Older Windows builds may ship SQLite without VACUUM INTO.
		// Fall back to a single-process WAL checkpoint followed by a copied file.
		if e := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); e != nil {
			return "", fmt.Errorf("cannot checkpoint database before backup: %w", e)
		}
		in, e := os.Open(dbPath)
		if e != nil {
			return "", e
		}
		defer in.Close()
		out, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = io.Copy(out, in)
		if se := out.Sync(); e == nil {
			e = se
		}
		if ce := out.Close(); e == nil {
			e = ce
		}
		if e != nil {
			os.Remove(path)
			return "", fmt.Errorf("copy backup: %w", e)
		}
	}
	return path, nil
}

func (s *Service) payrollSummary(r *http.Request) (any, error) {
	period, e := periodOrCurrent(r.URL.Query().Get("period"))
	if e != nil {
		return nil, e
	}
	runs, e := s.rows(`SELECT pr.id,le.name AS company,pr.pay_period,pr.status,pr.rule_version,COUNT(pi.id) AS items,
 SUM(CASE WHEN pi.component_id='PAY-BASE' THEN pi.amount_minor ELSE 0 END) AS sample_base_pay_minor
 FROM payroll_run pr JOIN legal_entity le ON le.id=pr.legal_entity_id LEFT JOIN payroll_item pi ON pi.run_id=pr.id
 WHERE pr.tenant_id=? GROUP BY pr.id ORDER BY pr.pay_period DESC`, tenantID)
	if e != nil {
		return nil, e
	}
	agreements, e := s.rows(`SELECT a.id,ee.person_id,p.display_name,a.currency,a.base_pay_minor,a.valid_from,a.source_ref
 FROM compensation_agreement a JOIN employment_episode ee ON ee.id=a.employment_id JOIN person p ON p.id=ee.person_id
 WHERE a.tenant_id=? ORDER BY ee.person_id LIMIT 80`, tenantID)
	if e != nil {
		return nil, e
	}
	counts := map[string]int{}
	// Payroll items do not carry tenant_id, so count them through payroll_run.
	for _, table := range []string{"compensation_agreement", "tax_profile", "social_insurance_period", "benefit_claim"} {
		n, e := count(s, "SELECT COUNT(*) AS n FROM "+table+" WHERE tenant_id=?", tenantID)
		if e != nil {
			return nil, e
		}
		counts[table] = n
	}
	n, e := count(s, "SELECT COUNT(*) AS n FROM payroll_item WHERE run_id IN (SELECT id FROM payroll_run WHERE tenant_id=?)", tenantID)
	if e != nil {
		return nil, e
	}
	counts["payroll_item"] = n
	return map[string]any{"period": period, "runs": runs, "agreements": agreements, "counts": counts, "calculation_mode": "synthetic_source_only_no_statutory_calculation"}, nil
}
func (s *Service) benefitList() (any, error) {
	claims, e := s.rows(`SELECT b.id,b.person_id,p.display_name,b.benefit_type,b.application_date,b.status,b.rule_ref FROM benefit_claim b JOIN person p ON p.id=b.person_id WHERE b.tenant_id=? ORDER BY b.application_date DESC,b.id LIMIT 100`, tenantID)
	if e != nil {
		return nil, e
	}
	return map[string]any{"items": claims}, nil
}
func (s *Service) benefitAdd(person, kind, date string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if _, e := requireDate(date); e != nil {
		return nil, e
	}
	if !in(kind, "sickness", "maternity", "occupational_injury", "other") {
		return nil, fail(422, "Unsupported benefit type")
	}
	return s.transaction(func() (any, error) {
		id := uid("BEN")
		if e := s.db.Exec(`INSERT INTO benefit_claim(id,tenant_id,person_id,benefit_type,application_date,status,rule_ref) VALUES (?,?,?,?,?,'pending','UNVERIFIED-DEMO')`, id, tenantID, person, kind, date); e != nil {
			return nil, e
		}
		if e := s.audit("benefit.claim", person, map[string]any{"kind": kind, "date": date}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "status": "pending", "person_id": person}, nil
	})
}
func (s *Service) immigrationSummary(r *http.Request) (any, error) {
	d, e := argDate(r)
	if e != nil {
		return nil, e
	}
	nationalities, e := s.rows(`SELECT nationality,COUNT(*) AS people FROM person WHERE tenant_id=? GROUP BY nationality ORDER BY people DESC`, tenantID)
	if e != nil {
		return nil, e
	}
	foreigners, e := s.rows(`SELECT p.id,p.display_name,p.nationality,w.authorization_type,w.status AS permit_status,w.valid_from,w.valid_to,w.role_scope,w.worksite_scope,
 td.document_type,td.masked_reference,td.expiry_date,ic.credential_type,ic.valid_to AS residence_valid_to
 FROM person p LEFT JOIN work_authorization w ON w.person_id=p.id AND w.tenant_id=p.tenant_id
 LEFT JOIN travel_document td ON td.person_id=p.id AND td.tenant_id=p.tenant_id
 LEFT JOIN immigration_credential ic ON ic.person_id=p.id AND ic.tenant_id=p.tenant_id
 WHERE p.tenant_id=? AND p.nationality<>'VN' ORDER BY p.id LIMIT 150`, tenantID)
	if e != nil {
		return nil, e
	}
	soon, _ := time.Parse("2006-01-02", d)
	end := soon.AddDate(0, 0, 90).Format("2006-01-02")
	expiring, e := count(s, "SELECT COUNT(*) AS n FROM work_authorization WHERE tenant_id=? AND valid_to BETWEEN ? AND ?", tenantID, d, end)
	if e != nil {
		return nil, e
	}
	return map[string]any{"as_of": d, "nationalities": nationalities, "experts": foreigners, "expiring_90d": expiring, "verification": "Synthetic, not legal authorization"}, nil
}
