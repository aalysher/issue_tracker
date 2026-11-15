---
name: agent-task
description: Use when turning a feature request into an agent-ready issue, acceptance criteria, or bounded implementation plan.
---

# Shape an agent-ready task

1. Restate the user-visible outcome and why it matters; do not invent product behavior.
2. Read the affected code and tests before proposing a solution.
3. Separate required behavior from explicit non-goals.
4. Make acceptance criteria observable, including an important invalid or denied path.
5. Name affected boundaries and compatibility constraints, but avoid prescribing internals without evidence.
6. State the smallest useful test and verification commands.
7. Identify ambiguities that would change behavior and ask before implementing unless a safe, clearly stated default exists.
8. Keep the task to one cohesive outcome. Split independent outcomes; do not parallelize work that depends on one continuous investigation.
