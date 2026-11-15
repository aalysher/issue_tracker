# Repository guidance

This is a small Go modular monolith with a React/TypeScript client. Keep changes scoped, preserve the API contract, and prefer standard-library Go unless a dependency clearly reduces risk or complexity.

## Required workflow

1. Read the relevant code, tests, API spec, and applicable skill before editing.
2. For multi-file or behavior-changing work, state observable acceptance criteria and make a short implementation plan.
3. Implement the smallest cohesive change. Do not silently relax validation, authorization, or tests to make a task pass.
4. Add or update focused tests. Run `gofmt`, `go test ./cmd/... ./internal/...`, and the frontend lint/test/build commands relevant to your change.
5. Review the diff for authorization boundaries, secrets, migration safety, accessibility, and accidental scope growth before reporting.

## Engineering constraints

- Every project-owned resource query must enforce ownership in the SQL operation itself.
- Keep credentials and session tokens out of logs and responses. Never commit secrets; `.env.example` contains local-only values.
- Use additive, versioned database migrations and make failures explicit.
- Keep client writes same-origin and send the required `X-Requested-With` header.
- API errors use the `{"error":{"code":"...","message":"..."}}` shape. Do not expose SQL or internal error details to clients.
- User-facing UI must preserve semantic labels, keyboard access, and visible error/loading feedback.
