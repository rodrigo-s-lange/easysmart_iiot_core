# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [2026-02-08]
### Added
- EMQX ACLs per device via PostgreSQL; `no_match = deny`.
- Docker log rotation for EMQX and API containers.
- Runtime notes and validation commands in README.

### Changed
- EMQX auth/ACL queries wired to PostgreSQL.
- `docker-compose.yml` now loads `.env` via `env_file`.
