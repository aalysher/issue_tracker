---
name: testing
description: Use when designing or updating tests for a feature or bug fix in this repository.
---

# Verification procedure

1. Convert each acceptance criterion into an observable assertion.
2. Prefer focused tests that fail on the old behavior and cover one meaningful boundary.
3. Test through the public package/API where practical; avoid tests that only restate implementation details.
4. Keep integration dependencies explicit. If PostgreSQL or a browser is unavailable, state that limitation and run all checks that are available.
5. Run the smallest relevant test set first, then the required Go/frontend checks for the changed surfaces.
6. Report exact commands and outcomes; do not mask failures or alter acceptance criteria to make tests pass.
