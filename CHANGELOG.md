# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
### Added
- TimescaleDB service and init schema for telemetria.
- TimescaleDB env vars in `.env`.
- Reset script to keep only the test device.
- Go API healthcheck in docker-compose.
- TimescaleDB retention policy (365 days).
- Rate limit no Go API via Redis (device + slot).
- Cache do último valor por slot via Redis e endpoint de leitura.
- Device provisioning flow implemented (HMAC bootstrap/secret + claim code + reset).
- Provisioning endpoints: `POST /api/devices/bootstrap`, `POST /api/devices/secret`, `POST /api/devices/reset`.
- Provisioning migration `003_device_provisioning_claim_code.sql` (adds `claim_code_hash`, `secret_delivered_at`).
- Env vars for provisioning: `MANUFACTURING_MASTER_KEY`, `BOOTSTRAP_MAX_SKEW_SECS`.
- `.env.example` added with required JWT secret note.
- Input validation via validator v10.
- Request ID middleware (`X-Request-ID`).
- Health endpoints: `/health/live` and `/health/ready`.
- Panic recovery middleware (JSON 500).
- Structured logging (slog).
- Graceful shutdown (configurable timeout).
- Prometheus metrics endpoint (`/metrics`) with basic counters.
- Direct provisioning endpoint for logged-in users: `POST /api/devices/provision`.
- MQTT broker config exposed for provisioning response: `MQTT_BROKER_HOST`, `MQTT_BROKER_PORT`.
- Migration runner script with tracking table support: `database/migrate.sh`.
- Basic Go test suite for auth/middleware/devices/telemetry utility flows.
- EMQX bootstrap reconciler service (`emqx_bootstrap`) to auto-restore webhook connector/action/rule after restarts.
- New env var `EMQX_WEBHOOK_API_KEY` in `.env.example` for telemetry webhook authentication.
- Observability stack in Docker Compose: Prometheus, Blackbox Exporter, Alertmanager, Grafana.
- Prometheus alert rules for API down, readiness failing and 5xx spike.
- Dedicated observability documentation: `docs/OBSERVABILITY.md`.
- Reinforcement tests for P0 item 1:
  - `go-api/handlers/auth_handler_test.go`
  - `go-api/handlers/ratelimiter_test.go`
  - `go-api/handlers/telemetry_utils_test.go`
- Production validation scripts:
  - `scripts/audit/test_tenant_isolation.sh`
  - `scripts/audit/test_e2e_mqtt_ingest.sh`
  - `scripts/audit/run_load_test.sh`
  - `scripts/audit/load_telemetry_k6.js`
  - `scripts/audit/security_smoke.sh`
  - `database/backup_restore.sh`
- Production validation runbook: `docs/PRODUCTION_VALIDATION.md`.

### Changed
- Go API grava telemetria no TimescaleDB (mantém auth no PostgreSQL).
- README/STATUS atualizados (topics tenant-aware, device_label vs device_id, provisioning Option A).
- Go API now refuses to start if `JWT_SECRET` is default/empty.
- Refresh token now uses current role/tenant/email from DB when issuing new tokens.
- Consolidated multi-tenant tables: `users_v2`/`devices_v2` renamed to `users`/`devices` with RLS enabled. Legacy tables preserved as `users_legacy`/`devices_legacy`.
- TimescaleDB telemetry agora inclui `tenant_id` com RLS (isolamento por tenant).
- Removed EMQX listener rate limiting (managed in Go API instead).
- `/api/devices/secret` no longer reissues secret when cache key is missing; retrieval is strictly one-time.
- Telemetry read endpoints (`/api/telemetry/latest`, `/api/telemetry/slots`) now require JWT + `telemetry:read` and tenant scoping.
- Telemetry webhook now validates tenant in MQTT topic against device tenant.
- Auth rate limiter now handles Redis=nil safely.
- API key middleware now rejects short keys safely (no panic).
- Register flow now serializes first-user bootstrap via advisory lock (avoids multi-super-admin race).
- Normalized migration ordering (`003/004/005`) and moved old single-tenant schema to `database/migrations/legacy/`.
- `docs/ROADMAP_P0_P2.md` now includes explicit execution sequencing for P1 (P1.1 to P1.5).
- Route hardening by explicit HTTP methods (invalid methods now return `405`).
- Device reset now requires `devices:provision` permission (lifecycle scope alignment).
- Telemetry read selectors now reject ambiguous requests that send both `device_id` and `device_label`.

### Docs
- Documented CORS behavior and configuration.
- Added short-term implementation roadmap (observability, operational security, provisioning tests/reset).
- OpenAPI updated with direct provisioning endpoint and telemetry read auth requirements.
- README rewritten in lighter format and aligned with current provisioning/security behavior.

### Removed
- Express API e serviço `nextjs` do compose (Go API agora é o único ingest).

## [2026-02-08]
### Added
- EMQX ACLs per device via PostgreSQL; `no_match = deny`.
- Docker log rotation for EMQX and API containers.
- Runtime notes and validation commands in README.

### Changed
- EMQX auth/ACL queries wired to PostgreSQL.
- `docker-compose.yml` now loads `.env` via `env_file`.
