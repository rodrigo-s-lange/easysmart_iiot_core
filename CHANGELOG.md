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
- Planned device provisioning flow documented in README (unclaimed → claim → secret).
- `.env.example` added with required JWT secret note.

### Changed
- Go API grava telemetria no TimescaleDB (mantém auth no PostgreSQL).
- README atualizado com stack e comandos do TimescaleDB.
- Go API now refuses to start if `JWT_SECRET` is default/empty.
- Refresh token now uses current role/tenant/email from DB when issuing new tokens.

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
