# OrgAxis v0.8 P0 increment — durable device pull checkpoint (draft)

This branch is based on `feature/time-attendance-core-v07`; it is **not a production-ready attendance/payroll release**.

## Implemented

- One-page vendor-neutral JSON pull using `timecore.HTTPPullReader`, configured only through server-side environment variables.
- `oa_time_pull_checkpoint` stores the opaque upstream cursor and last successful page per tenant/source/device in existing SQLite.
- Raw event import, quarantine updates, checkpoint advancement and audit record share a single SQLite transaction. A conflicting key rolls back the whole page, including the cursor.
- Duplicate event keys remain idempotent. SQLite UPDATE/DELETE triggers reject changes to accepted raw punches. A privileged database owner could still remove these triggers; no WORM claim.
- Additive migration and automatic `VACUUM INTO` snapshot `orgaxis-before-v08-*.sqlite3` before new schema is installed. Existing DB is retained.
- HR/ADMIN only for adapter state and pull; all `/v2` mutations require valid session and CSRF token.

## Integration contract

On the **server host** before starting OrgAxis:

```powershell
$env:ORGAXIS_DEVICE_SOURCE = "EDGE-JSON"
$env:ORGAXIS_DEVICE_ID = "SIM-GATE-01"
$env:ORGAXIS_DEVICE_ENDPOINT = "https://your-managed-gateway.example/api/punches"
$env:ORGAXIS_DEVICE_TOKEN = "<securely-provisioned-token>"
```

The example host is **not** a tested endpoint. Provision the actual device in `oa_time_device` and effective-dated `oa_badge_binding` using an approved administrative process before syncing. The synthetic `SIM-GATE-01` exists only on pilot fixtures; real machine provisioning UI/adapter is not provided.

- `GET /v2/time/adapter/state` — current cursor state, no secrets.
- `POST /v2/time/adapter/pull` — fetch exactly one bounded upstream page, atomically import it and commit its nonempty `next_cursor`. No URL, token or person assignment accepted from request payload.
- Upstream `GET ?cursor=...` returns `{"events":[{"event_id":"...","badge_id":"...","direction":"IN","occurred_at":"2026-11-18T08:00:00+07:00"}],"next_cursor":"stable-opaque-next"}`.
- The vendor's `event_id` and `next_cursor` must be stable across retries; nonempty pages must advance the cursor. This contract must be implemented by a gateway if the manufacturer's API cannot provide it.
- HTTP allowed only for loopback lab, HTTPS required otherwise. Redirects disabled. An upstream non-200 response does not advance cursor.
- Run the next pull explicitly; this release does not add automatic polling or dedicated device service accounts.

## Validation

```sh
cd go
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o OrgAxisServer.exe .
```

Automated tests cover UPDATE/DELETE trigger denial, rerunning DDL, vendor JSON page pull, checkpoint retention on 409 batch conflict, quarantine replay, empty page, simulated HTTP 503, RBAC and CSRF. Native Windows service, device clock drift, 8-hour disconnection, actual biometric hardware, service-account provisioning and restoring a real existing database are **not tested** here.

## Deployment guard

Stop the service/Tray, back up the entire data folder and keep the database file. Start on a **copy** of a production-like SQLite DB for migration/restore tests. The migration makes a separate snapshot once on first startup; protect it with restricted OS ACL and external backup policy.

**Blocked P0:** site timezone and device mapping ownership; leave/OT approved evidence; roster completeness and calendar; monthly close/reopen with source digest; Vietnam statutory payroll, BHXH/PIT; two-period reconciliation; security and independent AppGate UAT. Do not post payroll from `pay-preview`.


## Correctness gate added in v0.8

Effective-dated badge identity resolution uses the **device's site timezone**, not the timezone string supplied by a gateway. Existing device records receive `timezone='Asia/Ho_Chi_Minh'` by additive migration; a per-device IANA zone can be configured when site governance is established. The build embeds IANA timezone data for consistent Windows behavior. A test covers an event received in UTC after 17:00 that belongs to the next Vietnam local day.

A device response with multiple JSON documents, a credential-bearing URL or malformed JSON is rejected. Upstream errors are sanitized in HTTP responses. Cursor semantics and physical device event IDs remain gateway-dependent and unverified.
