//go:build linux && cgo

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPayrollWorkbenchLedgerAndTax(t *testing.T) {
	s := testService(t)
	for _, path := range []string{
		"/v1/payroll/workbench?period=2026-10&limit=10",
		"/v1/payroll/payslip/EMP-0001?period=2026-10",
		"/v1/payroll/simulate?gross=30000000&insurable_base=19000000&dependents=1",
	} {
		status, b := serve(t, s, "GET", path, "", true)
		if status != 200 {
			t.Fatalf("%s -> %d %s", path, status, b)
		}
	}
	status, b := serve(t, s, "GET", "/v1/payroll/workbench?period=2026-10&limit=10", "", true)
	d := obj(t, b)
	if status != 200 || int(d["total"].(float64)) != 1000 {
		t.Fatalf("ledger count %s", b)
	}
	if len(d["units"].([]any)) < 5 || len(d["trend"].([]any)) != 3 {
		t.Fatal("missing group distribution or month trends")
	}
	_, b = serve(t, s, "GET", "/v1/payroll/payslip/EMP-0001?period=2026-10", "", true)
	p := obj(t, b)["person"].(map[string]any)
	sum := 0.0
	for _, key := range []string{"base_pay", "allowance", "overtime", "attendance_bonus", "performance_bonus"} {
		sum += p[key].(float64)
	}
	if sum != p["gross"].(float64) {
		t.Fatal("gross does not reconcile")
	}
	if p["net_estimate"].(float64) != sum-p["employee_insurance"].(float64)-p["other_deduction"].(float64)-p["pit_estimate"].(float64) {
		t.Fatal("net does not reconcile")
	}
	for _, bad := range []string{"/v1/payroll/simulate?gross=-1&insurable_base=0&dependents=0", "/v1/payroll/simulate?gross=100&insurable_base=900&dependents=0", "/v1/payroll/simulate?gross=100&insurable_base=50&dependents=-2"} {
		status, _ := serve(t, s, "GET", bad, "", true)
		if status != 422 {
			t.Fatalf("bad tax input expected 422 got %d", status)
		}
	}
}
func TestForeignStaffNamesAndLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath, e := setupDB(filepath.Join(dir, "data"))
	if e != nil {
		t.Fatal(e)
	}
	db, e := openSQLite(dbPath)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e := db.Exec(`DROP TABLE payroll_demo_ledger`); e != nil {
		t.Fatal(e)
	}
	if e := db.Exec(`UPDATE person SET display_name='Nguyễn Văn Test (Demo 0120)' WHERE id='EMP-0120'`); e != nil {
		t.Fatal(e)
	}
	backup, e := backupBeforeWorkbench(db, dbPath)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(backup); e != nil {
		t.Fatal("missing backup", e)
	}
	if e := upgradePayrollWorkbench(db); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query(`SELECT COUNT(*) AS n FROM payroll_demo_ledger`)
	if e != nil {
		t.Fatal(e)
	}
	if intval(rows[0]["n"]) != 12000 {
		t.Fatal("missing 12000 ledger rows")
	}
	r, e := first(db, `SELECT display_name FROM person WHERE id='EMP-0120'`)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(rowString(r, "display_name"), "Nguyễn") || !strings.Contains(rowString(r, "display_name"), "Tanaka") {
		t.Fatalf("foreign identity not upgraded: %s", rowString(r, "display_name"))
	}
}
