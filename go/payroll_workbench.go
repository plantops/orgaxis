package main

import (
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Every numeric rule here is demonstrative, not a Vietnamese tax or insurance
// rule. A future statutory engine must use verified, versioned legal rule sets.
const demoPayrollRule = "ILLUSTRATIVE-2026-NOT-STATUTORY"

func demoTax(taxable int64) int64 {
	if taxable <= 0 {
		return 0
	}
	// Illustrative progressive bands, NOT a statutory PIT schedule.
	widths := []int64{5_000_000, 5_000_000, 8_000_000, 14_000_000, 20_000_000, 28_000_000}
	rates := []float64{.05, .10, .15, .20, .25, .30, .35}
	remaining := taxable
	var total float64
	for i, width := range widths {
		take := remaining
		if take > width {
			take = width
		}
		total += float64(take) * rates[i]
		remaining -= take
		if remaining <= 0 {
			return int64(math.Round(total))
		}
	}
	return int64(math.Round(total + float64(remaining)*rates[len(rates)-1]))
}

func demoLedger(i, month int, base int64, deps int) []int64 {
	allowance := int64(450_000 + (i%7)*130_000)
	overtime := int64(((i*7 + month) % 9) * 115_000)
	attendance := int64(0)
	if i%11 != 0 {
		attendance = 350_000
	}
	performance := int64(((i*13 + month) % 12) * 145_000)
	if i%25 == 0 {
		performance += 1_500_000
	}
	gross := base + allowance + overtime + attendance + performance
	insurance := int64(math.Round(float64(base) * .105)) // illustrative
	other := int64((i % 4) * 50_000)
	taxable := gross - insurance - 11_000_000 - int64(deps)*4_400_000 // illustrative deductions
	if taxable < 0 {
		taxable = 0
	}
	pit := demoTax(taxable)
	net := gross - insurance - other - pit
	return []int64{base, allowance, overtime, attendance, performance, gross, insurance, other, taxable, pit, net}
}

func upgradePayrollWorkbench(db SQLiteDB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS payroll_demo_ledger (
          tenant_id TEXT NOT NULL,
          person_id TEXT NOT NULL REFERENCES person(id),
          period TEXT NOT NULL,
          base_pay INTEGER NOT NULL, allowance INTEGER NOT NULL, overtime INTEGER NOT NULL,
          attendance_bonus INTEGER NOT NULL, performance_bonus INTEGER NOT NULL,
          gross INTEGER NOT NULL, employee_insurance INTEGER NOT NULL,
          other_deduction INTEGER NOT NULL, taxable_income INTEGER NOT NULL,
          pit_estimate INTEGER NOT NULL, net_estimate INTEGER NOT NULL,
          dependents INTEGER NOT NULL, rule_version TEXT NOT NULL,
          PRIMARY KEY(tenant_id, person_id, period)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_demo_pay_period ON payroll_demo_ledger(tenant_id,period)`,
	}
	for _, sql := range stmts {
		if e := db.Exec(sql); e != nil {
			return fmt.Errorf("workbench schema: %w", e)
		}
	}
	ex, e := first(db, `SELECT COUNT(*) AS n FROM payroll_demo_ledger WHERE tenant_id=? AND period='2026-10'`, tenantID)
	if e != nil {
		return e
	}
	if intval(ex["n"]) >= 1000 {
		return nil
	}
	rows, e := db.Query(`SELECT p.id,c.base_pay_minor,t.dependent_count FROM person p
      JOIN employment_episode ee ON ee.person_id=p.id AND ee.tenant_id=p.tenant_id
      JOIN compensation_agreement c ON c.employment_id=ee.id AND c.tenant_id=p.tenant_id
      JOIN tax_profile t ON t.person_id=p.id AND t.tenant_id=p.tenant_id
      WHERE p.tenant_id=? AND p.synthetic=1 ORDER BY p.id`, tenantID)
	if e != nil {
		return e
	}
	if len(rows) < 1000 {
		return nil
	} // never fabricate compensation on an incomplete or real tenant
	if e = db.Exec("BEGIN IMMEDIATE"); e != nil {
		return e
	}
	committed := false
	defer func() {
		if !committed {
			_ = db.Exec("ROLLBACK")
		}
	}()
	for _, person := range rows {
		id := rowString(person, "id")
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "EMP-"))
		base := int64(intval(person["base_pay_minor"]))
		deps := intval(person["dependent_count"])
		for _, month := range []int{9, 10, 11} {
			vals := demoLedger(n, month, base, deps)
			period := fmt.Sprintf("2026-%02d", month)
			e = db.Exec(`INSERT OR IGNORE INTO payroll_demo_ledger(tenant_id,person_id,period,base_pay,allowance,overtime,attendance_bonus,performance_bonus,gross,employee_insurance,other_deduction,taxable_income,pit_estimate,net_estimate,dependents,rule_version)
              VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tenantID, id, period,
				vals[0], vals[1], vals[2], vals[3], vals[4], vals[5], vals[6], vals[7], vals[8], vals[9], vals[10], deps, demoPayrollRule)
			if e != nil {
				return fmt.Errorf("workbench seed %s: %w", id, e)
			}
		}
	}
	// International identities are wholly fictional; repair early synthetic seed names only.
	names := []string{
		"Haruto Tanaka", "Li Wei", "Chen Yu-Hsuan", "Kim Min-jun", "Arjun Mehta", "Daniel Miller",
		"Yuki Nakamura", "Zhang Yiming", "Lin Chia-Hao", "Park Seo-yeon", "Priya Sharma", "Emily Carter",
		"Kenji Sato", "Wang Xiaoyu", "Huang Pei-Ling", "Lee Ji-hoon", "Rohan Patel", "Michael Anderson",
	}
	for i, name := range names {
		id := fmt.Sprintf("EMP-%04d", i+120)
		e = db.Exec(`UPDATE person SET display_name=? WHERE tenant_id=? AND id=? AND synthetic=1 AND nationality<>'VN' AND display_name LIKE '%(Demo %'`, name+" (Demo)", tenantID, id)
		if e != nil {
			return e
		}
	}
	if e = db.Exec("COMMIT"); e != nil {
		return e
	}
	committed = true
	return nil
}

func (s *Service) payrollWorkbench(r *http.Request) (any, error) {
	period, e := periodOrCurrent(r.URL.Query().Get("period"))
	if e != nil {
		return nil, e
	}
	q := likeText(r.URL.Query().Get("q"))
	limit, e := takeInt(r, "limit", 40, 1, 150)
	if e != nil {
		return nil, e
	}
	offset, e := takeInt(r, "offset", 0, 0, 10000)
	if e != nil {
		return nil, e
	}
	summary, e := s.one(`SELECT COUNT(*) AS staff,SUM(gross) AS gross,SUM(performance_bonus+attendance_bonus) AS bonuses,
      SUM(overtime) AS overtime,SUM(employee_insurance) AS insurance,SUM(pit_estimate) AS pit,
      SUM(net_estimate) AS net,AVG(gross) AS avg_gross FROM payroll_demo_ledger WHERE tenant_id=? AND period=?`, tenantID, period)
	if e != nil {
		return nil, e
	}
	entries, e := s.rows(`SELECT l.person_id,p.display_name,p.nationality,l.base_pay,l.allowance,l.overtime,l.attendance_bonus,
      l.performance_bonus,l.gross,l.employee_insurance,l.other_deduction,l.taxable_income,l.pit_estimate,l.net_estimate,
      l.dependents,l.rule_version
      FROM payroll_demo_ledger l JOIN person p ON p.id=l.person_id
      WHERE l.tenant_id=? AND l.period=? AND (lower(p.display_name) LIKE ? OR lower(p.id) LIKE ?)
      ORDER BY p.id LIMIT ? OFFSET ?`, tenantID, period, q, q, limit, offset)
	if e != nil {
		return nil, e
	}
	total, e := count(s, `SELECT COUNT(*) AS n FROM payroll_demo_ledger l JOIN person p ON p.id=l.person_id
      WHERE l.tenant_id=? AND l.period=? AND (lower(p.display_name) LIKE ? OR lower(p.id) LIKE ?)`, tenantID, period, q, q)
	if e != nil {
		return nil, e
	}
	trends, e := s.rows(`SELECT period,COUNT(*) AS staff,SUM(gross) AS gross,SUM(performance_bonus+attendance_bonus) AS bonus,SUM(net_estimate) AS net
      FROM payroll_demo_ledger WHERE tenant_id=? GROUP BY period ORDER BY period`, tenantID)
	if e != nil {
		return nil, e
	}
	unitStats, e := s.rows(`SELECT u.code AS unit_code,u.name AS unit_name,COUNT(*) AS staff,SUM(l.gross) AS gross,
      SUM(l.performance_bonus+l.attendance_bonus) AS bonus,AVG(l.gross) AS average_gross
      FROM payroll_demo_ledger l
      JOIN assignment a ON a.person_id=l.person_id AND a.assignment_type='primary'
        AND a.valid_from <= (? || '-15') AND (a.valid_to IS NULL OR a.valid_to > (? || '-15'))
      JOIN position_placement pl ON pl.position_id=a.position_id
        AND pl.valid_from <= (? || '-15') AND (pl.valid_to IS NULL OR pl.valid_to > (? || '-15'))
      JOIN org_unit u ON u.id=pl.org_unit_id WHERE l.tenant_id=? AND l.period=?
      GROUP BY u.id ORDER BY gross DESC`, period, period, period, period, tenantID, period)
	if e != nil {
		return nil, e
	}
	return map[string]any{"period": period, "summary": summary, "entries": entries, "total": total, "limit": limit, "offset": offset,
		"trend": trends, "units": unitStats, "run_state": map[string]string{"2026-09": "reviewed_demo", "2026-10": "in_review_demo", "2026-11": "draft_demo"}[period],
		"assumptions": map[string]any{"rule_version": demoPayrollRule, "insurance_fraction": 0.105, "base_deduction": 11000000, "dependent_deduction": 4400000,
			"tax_bands":  "ILLUSTRATIVE: progressive 5%,10%,15%,20%,25%,30%,35%; not verified Vietnamese law",
			"disclaimer": "ALL VALUES ARE SYNTHETIC, NOT LEGAL PAYSLIPS OR STATUTORY PIT"}}, nil
}
func (s *Service) payrollPayslip(person, period string) (any, error) {
	if e := s.requirePerson(person); e != nil {
		return nil, e
	}
	if period == "" {
		period = "2026-10"
	}
	if _, e := periodOrCurrent(period); e != nil {
		return nil, e
	}
	row, e := s.one(`SELECT l.*,p.display_name,p.nationality FROM payroll_demo_ledger l
      JOIN person p ON p.id=l.person_id WHERE l.tenant_id=? AND l.person_id=? AND l.period=?`, tenantID, person, period)
	if e != nil {
		return nil, e
	}
	if row == nil {
		return nil, fail(404, "No synthetic payslip for this period")
	}
	return map[string]any{"person": row, "period": period, "source": "synthetic_scenario", "rule_version": demoPayrollRule}, nil
}
func (s *Service) payrollTaxSimulation(r *http.Request) (any, error) {
	gross, e := strconv.ParseInt(r.URL.Query().Get("gross"), 10, 64)
	if e != nil || gross < 0 || gross > 1_000_000_000 {
		return nil, fail(422, "gross must be 0..1000000000 VND")
	}
	deps, e := strconv.Atoi(r.URL.Query().Get("dependents"))
	if e != nil || deps < 0 || deps > 25 {
		return nil, fail(422, "dependents must be 0..25")
	}
	insured, e := strconv.ParseInt(r.URL.Query().Get("insurable_base"), 10, 64)
	if e != nil || insured < 0 || insured > gross {
		return nil, fail(422, "insurable_base must be 0..gross")
	}
	insurance := int64(math.Round(float64(insured) * .105))
	taxable := gross - insurance - 11000000 - int64(deps)*4400000
	if taxable < 0 {
		taxable = 0
	}
	tax := demoTax(taxable)
	return map[string]any{"gross": gross, "insurable_base": insured, "dependents": deps, "illustrative_insurance": insurance,
		"illustrative_taxable": taxable, "illustrative_pit": tax, "illustrative_net": gross - insurance - tax,
		"rule_version": demoPayrollRule, "legal_status": "simulation_only_not_statutory"}, nil
}

// A one-time snapshot for legacy demo databases before the new demo finance tables
// are added. This never overwrites or deletes a prior snapshot.
func backupBeforeWorkbench(db SQLiteDB, dbPath string) (string, error) {
	table, e := first(db, `SELECT name FROM sqlite_master WHERE type='table' AND name='payroll_demo_ledger'`)
	if e != nil {
		return "", e
	}
	if table != nil {
		rows, e := first(db, `SELECT COUNT(*) AS n FROM payroll_demo_ledger WHERE tenant_id=?`, tenantID)
		if e != nil {
			return "", e
		}
		if intval(rows["n"]) >= 1000 {
			return "", nil
		}
	}
	people, e := first(db, `SELECT COUNT(*) AS n FROM person WHERE tenant_id=? AND synthetic=1`, tenantID)
	if e != nil {
		return "", e
	}
	if intval(people["n"]) < 1000 {
		return "", nil
	}
	target := filepath.Join(filepath.Dir(dbPath), "orgaxis-before-v03-workbench-"+time.Now().Format("20060102-150405")+".sqlite3")
	if e := db.Exec(`VACUUM INTO ?`, target); e != nil {
		return "", fmt.Errorf("backup before v0.3 migration failed: %w", e)
	}
	return target, nil
}
