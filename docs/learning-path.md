# Learning path: work effectively with coding agents

Use the issue tracker as a repeated exercise, not a one-shot code-generation prompt. Each lesson can be completed in a small branch and pull request. Read [the repository instructions](../.github/copilot-instructions.md), then pick a real improvement, such as pagination or issue labels.

## What the harness is

The model is only one part of an agent system. The harness is the surrounding environment that makes work repeatable and bounded: repository instructions, available tools and permissions, context selection, skills, specialized agents, lifecycle hooks, and verification. A reliable harness makes the expected workflow and stopping conditions explicit and checks outcomes with tests rather than trusting a confident summary.

## 0. Establish a baseline

Start the app with `docker compose up --build`; open <http://localhost:8088>, register, create a project, add an issue, change its status, and leave a comment. Before assigning an agent a task, inspect the current branch and ensure the working tree is understood. Never ask an agent to overwrite unrelated local changes.

## 1. Give the agent a good task

Write a short issue with:

- the user-visible problem and why it matters;
- explicit in-scope and out-of-scope behavior;
- observable acceptance criteria, including failure cases;
- relevant API/UI surfaces and constraints;
- required tests and the command that proves the change.

Avoid prescribing an implementation before exploring the current design. Ask the agent to inspect the code and report a plan first for multi-file changes. Resolve unclear product behavior before code is written.

## 2. Treat instructions as durable harness configuration

`.github/copilot-instructions.md` contains repository-wide, always-relevant constraints. Keep it short and stable; put task-specific procedures in skills instead of making every conversation carry every detail. Instructions guide behavior, but are not a security boundary: still review tool permissions and diffs.

## 3. Use skills for repeatable procedures

Skills under `.github/skills/` are discovered by Copilot CLI and loaded when relevant. They should describe a bounded procedure with inputs, decisions, checks, and stop conditions. This repository's Go API, PostgreSQL migration, and testing skills are examples. A skill should not be a second copy of the entire project README.

In Copilot CLI, use `/skills` to inspect/manage available skills and `/env` to inspect what configuration was loaded. The equivalent `copilot skill list` shows project skills. Check the current CLI help/docs when command behavior changes between releases.

## 4. Delegate roles, not vague outcomes

Repository agent profiles live in `.github/agents/`. Use a backend specialist for a bounded API task, then a reviewer with a separate objective to look for defects. Provide each agent the issue, allowed files/scope, acceptance criteria, commands, and what it must not change. Delegation helps when work is genuinely independent; do not parallelize a single cross-cutting reasoning chain or ask multiple agents for duplicate reviews.

Use `/agent go-api`, `/agent frontend`, or `/agent code-reviewer` to select a profile. The CLI also accepts `copilot --agent <name>` when starting a session. Use `/review` for a review pass; `/fleet` is for multiple independent workstreams, not several agents tracing the same feature. Review the resulting changes yourself; agents do not own the merge decision.

## 5. Run the full loop

For a feature such as pagination:

1. Ask for repository exploration and a short plan with acceptance criteria. In the CLI, `/plan` starts the planning workflow.
2. Confirm API behavior and edge cases (ordering, page-size limits, invalid cursors) before implementation.
3. Implement a small vertical slice and add tests that fail before the change.
4. Run formatting, unit/integration tests available in the environment, lint, and build. Review the exact diff and failure output.
5. Ask a separate reviewer to search for correctness, authorization, error-handling, and regression risks; fix valid findings and rerun checks.
6. Summarize changed behavior, checks run, unresolved risks, and a manual test path. Keep the final approval and merge decision human-owned.

For this project, the baseline commands are `gofmt -w` for changed Go files, `go test ./cmd/... ./internal/...`, `go vet ./cmd/... ./internal/...`, and from `frontend/`: `npm run lint`, `npm test`, `npm run build`.

## 6. Measure results, not agent activity

Evaluate whether the acceptance criteria were met, tests cover behavior and boundaries, reviewers catch real defects, and the change is understandable. Track rework and escaped defects over several tasks; do not optimize for token counts, number of agents, or code volume. If failures recur, improve the task, harness, test coverage, or tool constraints at the source.

## Production and safety boundary

This repository demonstrates production-minded defaults, not a certified production deployment. Local Compose credentials are deliberately local-only. A real deployment needs a TLS-terminating trusted proxy, production secret management, backups and restore drills, monitoring/alerts, dependency and image update policy, and a review of organizational privacy/compliance requirements. Never give an agent production credentials or unrestricted authority over deployment and customer data.
