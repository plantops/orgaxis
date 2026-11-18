package main

import (
	"path/filepath"
	"time"
)

// backupBeforeAttendance is a one-time migration safety checkpoint. SQLite's
// VACUUM INTO creates a consistent copy including committed WAL content.
// It does not replace scheduled, encrypted off-machine backups and restore drills.
func backupBeforeAttendance(db SQLiteDB, dbPath string) (string, error) {
	r, e := first(db, `SELECT name FROM sqlite_master WHERE type='table' AND name='oa_time_roster'`)
	if e != nil || r != nil {
		return "", e
	}
	target := filepath.Join(filepath.Dir(dbPath), "orgaxis-before-v07-"+time.Now().UTC().Format("20060102-150405")+".sqlite3")
	if e = db.Exec(`VACUUM INTO ?`, target); e != nil {
		return "", e
	}
	return target, nil
}
