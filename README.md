# OrgAxis — Temporal Organization & Workforce Engine

API-first demonstration of time-aware organizational reporting and workflow resolution.

## Scope

Current implementation is a **synthetic-data demonstration**, not a production HRIS or statutory calculation engine.
- 1,000 synthetic employees
- Exactly 2 blank organization changes (2026-07-01 and 2027-01-01)
- Exactly 80 workforce transfer events
- As-of-date manager lookup, reporting chain, blank/occupied org chart, and workflow routing
- Data-model placeholders for skills, training, payroll, benefits, tax, and immigration

## Run

```bash
docker compose up --build -d
```

Demo binds `127.0.0.1:8080` only. The default API key is for a local demo; do not expose the demo over a public network.

```bash
curl -H 'X-API-Key: demo-local-key' \
 'http://127.0.0.1:8080/v1/people/EMP-0030/manager?as_of=2026-02-01'
curl -H 'X-API-Key: demo-local-key' \
 'http://127.0.0.1:8080/v1/people/EMP-0030/manager?as_of=2026-11-18'
curl -H 'X-API-Key: demo-local-key' \
 'http://127.0.0.1:8080/v1/org/blank?as_of=2027-01-01'
```

Web UI: http://127.0.0.1:8080/

API docs: http://127.0.0.1:8080/docs

To run outside Docker:

```bash
python -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
python seed.py --db demo.sqlite3
uvicorn api:app --host 127.0.0.1 --port 8080
```

Run tests:

```bash
pip install pytest httpx
python -m pytest -q
```

## API endpoints

- `GET /health`
- `GET /v1/org/blank?as_of=YYYY-MM-DD`
- `GET /v1/org/occupied?as_of=YYYY-MM-DD`
- `GET /v1/org/changes`
- `GET /v1/people/{person_id}/manager?as_of=YYYY-MM-DD`
- `GET /v1/people/{person_id}/chain?as_of=YYYY-MM-DD`
- `GET /v1/people/{person_id}/reports?as_of=YYYY-MM-DD`
- `GET /v1/workflow/resolve?subject_id=...&levels=1&as_of=YYYY-MM-DD`
- `POST /v1/assignments/move`
- `POST /v1/org/reporting/change`

## Implementation boundaries

The API uses a single demonstration tenant and API key; **it is not tenant-isolated**.
Production mode is intentionally blocked. Production readiness requires OIDC/RBAC,
tenant scoping on every read/write, database migrations, temporal concurrency controls,
authorization policies, stronger audit and evidence integrity, privacy review, and load testing.

Every employee/name in the seed is fabricated. Real employee data must not be imported into this demo.


## One-company tenant pilot

A demo deployment is bound to one tenant ID and one database file. This is **not**
shared-database multi-tenancy or hosted SaaS. The `compose.yaml` creates an isolated
sample tenant `tenant-cfc-pilot` and company `CFCPILOT` on the first run, with
1,000 synthetic employees, 2 blank-org changes and 80 workforce changes.

```bash
# Local demo only; bind port to loopback, never expose default credentials publicly
export DEMO_API_KEY=demo-local-key
# Use docker compose rather than the old docker-compose tool
# From a fresh database volume:
docker compose up --build -d
curl -H 'X-API-Key: demo-local-key' http://127.0.0.1:8080/v1/tenant
curl -H 'X-API-Key: demo-local-key' http://127.0.0.1:8080/v1/companies
curl -H 'X-API-Key: demo-local-key' \
  'http://127.0.0.1:8080/v1/people/EMP-0030/manager?as_of=2026-11-18'
```

Companies can be created via `POST /v1/companies` with a JSON body such as:

```json
{"code":"SECOND","name":"Second Example Company (SYNTHETIC)","jurisdiction":"VN"}
```

Only metadata is created by that endpoint. Organizational structure, positions,
people and employment assignments are separate operations.

**Critical:** switching `ORGAXIS_TENANT_ID` requires a separate database/volume.
Do not switch the ID against a populated database, and do not expose this demo
API to the internet. Tenant isolation is by deployment/database, not a complete
hosted multi-tenant security implementation.
