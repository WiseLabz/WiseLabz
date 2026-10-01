# Tasks

## 1. Shared health check and job

- [x] 1.1 Extract `RunHealthCheck` from the Health handler and make the handler use it; verify existing handler tests pass unchanged
- [x] 1.2 Add store query for enabled connectors not in maintenance; verify store test on sqlite and postgres
- [x] 1.3 Add `health.cron_expr` config default and validation; verify config tests for default and invalid input
- [x] 1.4 Implement bounded-concurrency job with per-check timeout and register in `cmd/server/main.go`; verify tests for recording, maintenance skip, disabled skip and timeout isolation

## 2. Uptime math and APIs

- [x] 2.1 Make `GetConnectorUptime` exclude maintenance spans; verify store tests for outage inside, straddling and outside maintenance
- [x] 2.2 Add `GET /connectors/{id}/uptime/history` with bucketing; verify handler and store tests incl. authorization
- [x] 2.3 Add `GET /uptime` fleet aggregate with grant filtering; verify handler test with a hidden connector
- [x] 2.4 Update `docs/openapi.yaml` (endpoints, `uptime` widget type) and regenerate client with `bun run gen:api`; verify openapi contract test passes

## 3. Web

- [x] 3.1 Add availability/MTTR strip and sparkline to `ServiceDetailPage.tsx` with "no data" state and i18n keys; verify vitest with MSW
- [x] 3.2 Add fleet uptime dashboard widget, default disabled, in `widgets.tsx` and `store/dashboard.ts`; verify vitest incl. existing layouts unchanged

## 4. Integration

- [x] 4.1 Run backend and web test suites and `openspec validate scheduled-health-uptime`; run app locally and confirm rows accrue every interval; run `graphify update .`
