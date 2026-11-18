package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/plantops/orgaxis/timecore"
)

// This migration is additive. The existing source records are never recreated.
func backupBeforeCheckpoint(db SQLiteDB, dbPath string) (string, error) {
	exists, err := first(db, "SELECT name FROM sqlite_master WHERE type='table' AND name='oa_time_pull_checkpoint'")
	if err != nil || exists != nil {
		return "", err
	}
	target := filepath.Join(filepath.Dir(dbPath), "orgaxis-before-v08-"+time.Now().UTC().Format("20060102-150405.000000000")+".sqlite3")
	if err := db.Exec("VACUUM INTO ?", target); err != nil {
		return "", fmt.Errorf("backup before v0.8 attendance migration: %w", err)
	}
	return target, nil
}

func attendanceCheckpointSchema(db SQLiteDB) error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS oa_time_pull_checkpoint (
			tenant_id TEXT NOT NULL, source TEXT NOT NULL, device_id TEXT NOT NULL,
			cursor TEXT NOT NULL, last_success_utc TEXT NOT NULL,
			last_page_events INTEGER NOT NULL CHECK(last_page_events>=0),
			last_accepted INTEGER NOT NULL CHECK(last_accepted>=0),
			PRIMARY KEY(tenant_id,source,device_id))`,
		// Application-level append-only policy must also hold for accidental SQL writes.
		// The DB owner still has the ability to drop triggers; this is not WORM storage.
		`CREATE TRIGGER IF NOT EXISTS oa_raw_punch_immutable_update
			BEFORE UPDATE ON oa_raw_punch
			BEGIN SELECT RAISE(ABORT,'immutable raw punch: UPDATE denied'); END`,
		`CREATE TRIGGER IF NOT EXISTS oa_raw_punch_immutable_delete
			BEFORE DELETE ON oa_raw_punch
			BEGIN SELECT RAISE(ABORT,'immutable raw punch: DELETE denied'); END`,
	}
	for _, q := range ddl {
		if err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

type pullCheckpoint struct {
	Source, DeviceID, Cursor string
	PageEvents, Accepted int
}

func (s *Service) writePullCheckpoint(cp pullCheckpoint) error {
	if !sourceIdentifier.MatchString(cp.Source) || !sourceIdentifier.MatchString(cp.DeviceID) ||
		len(cp.Cursor) == 0 || len(cp.Cursor) > 1024 {
		return fail(422, "Invalid device checkpoint identity or cursor")
	}
	return s.db.Exec(`INSERT INTO oa_time_pull_checkpoint
		(tenant_id,source,device_id,cursor,last_success_utc,last_page_events,last_accepted)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(tenant_id,source,device_id) DO UPDATE SET
		  cursor=excluded.cursor,last_success_utc=excluded.last_success_utc,
		  last_page_events=excluded.last_page_events,last_accepted=excluded.last_accepted`,
		tenantID, cp.Source, cp.DeviceID, cp.Cursor,
		time.Now().UTC().Format(time.RFC3339Nano), cp.PageEvents, cp.Accepted)
}

// Only deployment-controlled environment variables configure the upstream URL
// and credentials. An authenticated HTTP caller cannot pass a URL (SSRF gate).
func configuredPull() (timecore.HTTPPullReader, string, string, error) {
	source, device := os.Getenv("ORGAXIS_DEVICE_SOURCE"), os.Getenv("ORGAXIS_DEVICE_ID")
	endpoint := os.Getenv("ORGAXIS_DEVICE_ENDPOINT")
	if !sourceIdentifier.MatchString(source) || !sourceIdentifier.MatchString(device) || endpoint == "" {
		return timecore.HTTPPullReader{}, "", "", fail(409, "Adapter not configured: source/device/endpoint")
	}
	return timecore.HTTPPullReader{Endpoint: endpoint, BearerToken: os.Getenv("ORGAXIS_DEVICE_TOKEN"),
		Client: &http.Client{Timeout: 10 * time.Second}}, source, device, nil
}

func (s *Service) pullStatus() (any, error) {
	_, source, device, err := configuredPull()
	if err != nil {
		return nil, err
	}
	row, err := s.one(`SELECT cursor,last_success_utc,last_page_events,last_accepted
		FROM oa_time_pull_checkpoint WHERE tenant_id=? AND source=? AND device_id=?`,
		tenantID, source, device)
	if err != nil {
		return nil, err
	}
	// Do not return endpoint, token, or synthetic employee data here.
	return map[string]any{"source": source, "device_id": device,
		"checkpoint": row, "configured": true}, nil
}

// One bounded page per explicit request. Checkpoint advancement and raw import
// share one SQLite transaction; HTTP outages and conflicts leave the cursor unchanged.
// Unattended polling/physical-device compatibility are separate acceptance gates.
func (s *Service) pullPage(ctx context.Context, actor string) (any, error) {
	reader, source, device, err := configuredPull()
	if err != nil {
		return nil, err
	}
	dev, err := s.one(`SELECT enabled FROM oa_time_device WHERE tenant_id=? AND device_id=?`, tenantID, device)
	if err != nil {
		return nil, err
	}
	if dev == nil || intval(dev["enabled"]) != 1 {
		return nil, fail(409, "Configured device is unknown or disabled")
	}
	checkpoint, err := s.one(`SELECT cursor FROM oa_time_pull_checkpoint
		WHERE tenant_id=? AND source=? AND device_id=?`, tenantID, source, device)
	if err != nil {
		return nil, err
	}
	cursor := rowString(checkpoint, "cursor")
	page, err := reader.Fetch(ctx, cursor)
	if err != nil {
		return nil, fail(502, "Device API read failed: "+err.Error())
	}
	if len(page.NextCursor) > 1024 {
		return nil, fail(422, "Device API returned oversized cursor")
	}
	if len(page.Events) > 0 && (page.NextCursor == "" || page.NextCursor == cursor) {
		return nil, fail(422, "Device API must advance a durable cursor for nonempty page")
	}
	if len(page.Events) == 0 && page.NextCursor == cursor {
		return map[string]any{"source": source, "device_id": device,
			"accepted": 0, "duplicate": 0, "exceptions": 0, "idle": true}, nil
	}
	if page.NextCursor == "" {
		return nil, fail(422, "Device API must return a durable cursor")
	}
	cp := &pullCheckpoint{Source: source, DeviceID: device, Cursor: page.NextCursor, PageEvents: len(page.Events)}
	if len(page.Events) == 0 {
		if err := s.db.Exec("BEGIN IMMEDIATE"); err != nil {
			return nil, err
		}
		committed := false
		defer func() {
			if !committed { _ = s.db.Exec("ROLLBACK") }
		}()
		if err := s.writePullCheckpoint(*cp); err != nil {
			return nil, err
		}
		if err := auditV6(s, actor, "TIME_DEVICE_CURSOR_ADVANCE", device,
			map[string]any{"source": source, "events": 0}); err != nil {
			return nil, err
		}
		if err := s.db.Exec("COMMIT"); err != nil {
			return nil, err
		}
		committed = true
		return map[string]any{"source": source, "device_id": device,
			"accepted": 0, "duplicate": 0, "exceptions": 0, "idle": true}, nil
	}
	return s.ingestPunchesWithCheckpoint(DeviceBatch{Source: source, DeviceID: device, Events: page.Events}, actor, cp)
}
