# Fieldnotes Issue Tracker

A compact Go + React issue tracker and a practical lab for harnesses, skills, coding agents, and the full agent-assisted development loop.

## Run locally

Requirements: Go 1.27+, Node 22+, npm, and Docker Compose.

```sh
docker compose up --build
```

Open <http://localhost:8088>, create an account, add a project, then create and discuss an issue. Stop with `Ctrl-C`; the local database persists in the `postgres-data` volume. `docker compose down -v` deletes that local database.

For direct development, run `docker compose up -d db`, then run `set -a; . ./.env; set +a; go run ./cmd/api` from the repository root after copying `.env.example` to `.env`. In another terminal run `cd frontend && npm ci && npm run dev`; Vite proxies `/api` to the Go server.

## Project map

- `cmd/api`, `internal/app`: Go HTTP API, auth/session handling, domain queries.
- `internal/database/migrations`: embedded sequential PostgreSQL migrations.
- `frontend`: React/TypeScript single-page application.
- `docs/openapi.yaml`: API contract.
- `.github/copilot-instructions.md`: repository-wide agent guidance.
- `.github/agents/`, `.github/skills/`: scoped role profiles and reusable procedures.
- `docs/learning-path.md`: lessons that use the app to practice planning, implementation, verification, and review.

## Checks

```sh
gofmt -w cmd internal
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
(cd frontend && npm ci && npm run lint && npm test && npm run build)
```

The normal Go test command skips the PostgreSQL integration test unless `TEST_DATABASE_URL` is set. With the local Compose database running, use:

```sh
TEST_DATABASE_URL='postgres://tracker:tracker-local-only@127.0.0.1:54329/tracker?sslmode=disable' go test -count=1 ./cmd/... ./internal/...
```

CI runs the PostgreSQL-backed test, Go static analysis and vulnerability scan, frontend tests/build, and npm dependency audit.

## Configuration and security

The API requires `DATABASE_URL`; see `.env.example` for local settings. Production should set `COOKIE_SECURE=true`, use TLS and a database connection with certificate verification (for example `sslmode=verify-full`), inject secrets from a secret manager, and restrict database network access. The Compose password and `COOKIE_SECURE=false` are for local development only. The app expects the browser and API to share an origin; do not enable permissive CORS.

This is a production-minded learning baseline, not a production certification. Before exposing it to real users, add operational controls for TLS termination, secrets, backups/restore, monitoring and alerting, image/dependency updates, and your privacy/compliance requirements. The Nginx auth rate limit uses the peer IP; behind a load balancer, configure trusted client-IP forwarding explicitly or enforce the limit at the edge. See [the learning path](docs/learning-path.md).
