# Time & Attendance API v0.7 prototype

**No live clock connected.** All data in the pilot is synthetic and limited to 02–07 November 2026. The source app is a single-tenant, local demo with generated credentials in `data/DEMO-LOGIN-PRIVATE.txt`.

## Endpoints

| Method/path | Roles | Behavior |
|---|---|---|
| `GET /time` | Public static page; fetches protected data | Time & Payroll Lab UI |
| `GET /v2/time/devices` | HR, Admin | List configured devices |
| `POST /v2/time/punches/import` | HR, Admin, session+CSRF | Batch import idempotent terminal events (up to 2,000) |
| `GET /v2/time/day?date=2026-11-04&person_id=EMP-0012` | Own, manager scope, HR/Admin | Daily source-to-timesheet valuation |
| `GET /v2/time/quality` | HR, Admin | Punch/roster counts, quarantine counts |
| `GET /v2/time/pay-preview?period=2026-11&person_id=EMP-0012` | Own, Payroll/Admin | Gross-only calculation preview, **BLOCKED** on missing periods or exceptions |

## Import example

```
POST /v2/time/punches/import
Content-Type: application/json
X-CSRF-Token: <token from /auth/login>
Cookie: oa_session=<session>

{
 "source": "CLOCK-EDGE-GATE",
 "device_id": "SIM-GATE-01",
 "events": [
  {"event_id":"device-unique-4234", "badge_id":"EMP-0012", "direction":"IN", "occurred_at":"2026-11-04T08:00:00+07:00"}
 ]
}
```

`source/device/event_id` is stable deduplication key. Timestamps must include timezone. Accepted events retain their original source and arrival time; conflicting replay rejects the entire batch. Unknown badge or direction is quarantined; subsequent replay of same quarantined event is idempotent and can be resolved after badge mapping correction. Employees cannot post or read unrelated time entries. This does NOT implement a machine-to-machine auth principal yet; the endpoint is for an authenticated HR-admin test only.

## Data movement

```
Vendor device API / site gateway
 -> API adapter (cursor + retries, not yet integrated with physical machines)
 -> event validation (timestamp, device, badge binding, direction)
 -> append-only raw punch / exception quarantine
 -> effective-dated shift & approved leave + OT windows
 -> timecore daily evaluation (block anomalies)
 -> month time ledger (period lock not yet implemented)
 -> gross preview only (NOT statutory payroll; incomplete period blocked)
```

## Known limitations

- No real device polling, employee sync, ZKTeco/Hanet/Hikvision/other protocol guarantee.
- No real holiday calendar, meal break timestamps, shift reassignment after schedule, overtime legal-ceiling rules, night pay, social insurance or tax computation.
- No retroactive correction, monthly time lock, approval-to-credit linkage from `oa_case`, tamper-evident immutable archive or service-account credentials.
- The time UI shows *synthetic* aggregates only. Do not connect production employee identifiers, passwords or live clock data to this pilot.
