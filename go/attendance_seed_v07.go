package main

import (
	"fmt"
	"time"
)

// seedAttendanceDemo is safe to run only with the SYNTHETIC pilot. It checks
// for existing roster records and never overwrites a customer's time records.
// It creates one simulated device and six days of attendance for 1,000 people.
func seedAttendanceDemo(db SQLiteDB) error {
	existing, e := first(db, `SELECT COUNT(*) AS n FROM oa_time_roster WHERE tenant_id=?`, tenantID)
	if e != nil {
		return e
	}
	if intval(existing["n"]) > 0 {
		return nil
	}
	if e = db.Exec("BEGIN IMMEDIATE"); e != nil {
		return e
	}
	committed := false
	defer func() {
		if !committed {
			_ = db.Exec("ROLLBACK")
		}
	}()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = db.Exec(`INSERT OR IGNORE INTO oa_time_device(tenant_id,device_id,site_id,description,source_type,enabled) VALUES(?,?,?,?,?,1)`, tenantID, "SIM-GATE-01", "PILOT-PLANT", "Synthetic sample terminal, no hardware connected", "SIMULATOR"); e != nil {
		return e
	}
	tz := time.FixedZone("UTC+07", 7*3600)
	for i := 1; i <= 1000; i++ {
		person := fmt.Sprintf("EMP-%04d", i)
		if e = db.Exec(`INSERT OR IGNORE INTO oa_badge_binding(tenant_id,device_id,badge_id,person_id,valid_from) VALUES(?,?,?,?,?)`, tenantID, "SIM-GATE-01", person, person, "2026-01-01"); e != nil {
			return e
		}
		for day := 2; day <= 7; day++ {
			startHour := 8
			if i%3 == 0 {
				startHour = 0
			} else if i%3 == 2 {
				startHour = 16
			}
			start := time.Date(2026, 11, day, startHour, 0, 0, 0, tz)
			end := start.Add(8 * time.Hour)
			breakMin := 30
			if startHour == 0 {
				breakMin = 45
			}
			workday := fmt.Sprintf("2026-11-%02d", day)
			if e = db.Exec(`INSERT INTO oa_time_roster VALUES(?,?,?,?,?,?,?)`, tenantID, person, workday, start.Format(time.RFC3339), end.Format(time.RFC3339), breakMin, "SYNTHETIC_SEED"); e != nil {
				return e
			}
			// Explicit leave credit on Nov 3 for a subset of workers; no biometric data.
			if day == 3 && i%113 == 0 {
				if e = db.Exec(`INSERT INTO oa_time_leave_credit VALUES(?,?,?,?,?,?,?)`, tenantID, person, fmt.Sprintf("SIM-LEAVE-%04d", i), start.Format(time.RFC3339), end.Format(time.RFC3339), 1, 1); e != nil {
					return e
				}
				continue
			}
			clockIn := start
			if i%127 == 0 {
				clockIn = clockIn.Add(15 * time.Minute)
			}
			clockOut := end
			if day == 4 && i%89 == 0 {
				clockOut = end.Add(time.Hour)
				if e = db.Exec(`INSERT INTO oa_time_ot VALUES(?,?,?,?,?,?,?)`, tenantID, person, fmt.Sprintf("SIM-OT-%04d", i), end.Format(time.RFC3339), clockOut.Format(time.RFC3339), "WEEKDAY", 1); e != nil {
					return e
				}
			} else if day == 4 && i%161 == 0 {
				clockOut = end.Add(time.Hour)
			}
			id := fmt.Sprintf("SIM-%04d-%02d", i, day)
			if e = db.Exec(`INSERT INTO oa_raw_punch VALUES(?,?,?,?,?,?,?,?,?)`, tenantID, "SIMULATOR", "SIM-GATE-01", id+"-IN", person, person, "IN", clockIn.UTC().Format(time.RFC3339Nano), now); e != nil {
				return e
			}
			if i%137 == 0 && day == 5 {
				continue
			} // Missing OUT: HR review exception
			if e = db.Exec(`INSERT INTO oa_raw_punch VALUES(?,?,?,?,?,?,?,?,?)`, tenantID, "SIMULATOR", "SIM-GATE-01", id+"-OUT", person, person, "OUT", clockOut.UTC().Format(time.RFC3339Nano), now); e != nil {
				return e
			}
		}
	}
	if e = db.Exec("COMMIT"); e != nil {
		return e
	}
	committed = true
	return nil
}
