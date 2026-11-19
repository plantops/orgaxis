# OrgAxis v0.11 — Functional Module Inventory & Engineering Maturity

**Status:** Source-reviewed synthetic pilot; Draft PR; not AppGate certification.
**Baseline:** v0.10 design at 475b2c9, with stacked v0.9 time ledger.
**Runtime:** Go 1.23 + SQLite, Windows 10 x64, local 127.0.0.1:8765, no Docker required.

## Additions

- 35 source-reviewed modules in 11 business/platform groups, each with name, purpose, function list, /5 maturity, implementation status, source evidence, open gaps.
- `GET /v2/modules`, signed-in session only, read-only:
  - `GET /v2/modules?group=ORG`
  - `GET /v2/modules?group=WFM&status=design_only`
- HR Workspace `/workspace` sidebar > **Module Inventory · v0.11**. Shows grouped full list and overview average.
- Automated source/integration tests (Linux CGO) for catalog auth, completeness, filter validation, score bounds and truthfulness.
- Full detailed report: `docs/ORGAXIS-MODULE-INVENTORY-v0.11.md`.

## Maturity definitions

0: absent; 1: specification only; 2: partial synthetic demo; 3: automated-test-backed synthetic pilot; 4: actual-data, integration, resilience and workflow acceptance; 5: independent production acceptance. Scores can have half points and are not AppGate scores.

## Boundaries

No new duty/RACI/coverage implementation, live attendance device or ERP, actual payroll, tax or BHXH calculation, government compliance determination, native Windows UAT or independent AppGate UAT was added.
Existing module `/v1/modules` is retained for compatibility; it is a legacy 12-module navigation subset. The authoritative v0.11 catalog is `/v2/modules`.

## Windows usage

Use on a **copy of the synthetic database**. Keep previous `data/` backups. Stop old tray and service, replace only both executables in your test installation, and open `http://127.0.0.1:8765/workspace`.
Use demo credentials already generated in `data/DEMO-LOGIN-PRIVATE.txt`. No credentials are embedded in release.

## Validation

From `go/` on a Linux CGO runner with seeded synthetic assets:

```sh
go test ./...
go test -race ./...
go vet ./...
node --check assets/workspace.js
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o OrgAxisServer.exe .
```

Passing CI proves source/build checks on a simulated pilot only. Windows native GUI/tray/service testing remains pending.

## Next engineering slices

P0: approved HR data source, site roster & leave/OT source evidence; complete source-to-pay with exception/correction/retro flow; 100-worker x 2-month HR/Finance reconciliation.
P0: effective-dated organizational structure import/validation (positions and reporting), function charter/RACI, versioned job descriptions.
P1: 24/7 staffing demand, labor policy, shift relief, handover and coverage gap warnings.
P1: independent security/backup UAT and AppGate evidence record. Never promote synthetic maturity to production by merely changing catalog numbers.
