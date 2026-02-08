# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
### Added
- TimescaleDB service and init schema for telemetria.
- TimescaleDB env vars in `.env`.
- Reset script to keep only the test device.

### Changed
- Go API grava telemetria no TimescaleDB (mantém auth no PostgreSQL).
- README atualizado com stack e comandos do TimescaleDB.

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
