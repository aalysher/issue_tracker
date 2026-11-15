---
name: go-api
description: Use when adding or changing Go HTTP API behavior in this repository.
---

# Go API change procedure

1. Inspect the existing route, handler, database query, API schema, and nearby tests.
2. Record success and failure behavior, including the authenticated user's ownership boundary.
3. Validate request sizes and fields; reject unknown JSON fields. Keep client errors stable and internal details in structured logs only.
4. Include ownership in the SQL operation; do not fetch an unrestricted object and authorize it later.
5. Add focused tests for the behavior and relevant invalid/unauthorized paths. Update `docs/openapi.yaml` when the contract changes.
6. Run `gofmt`, `go test ./cmd/... ./internal/...`, and `go vet ./cmd/... ./internal/...`. Stop and report the exact failure rather than hiding or weakening it.

Do not add dependencies, endpoints, or abstractions unrelated to the requested behavior.
