---
name: postgres-migrations
description: Use when changing PostgreSQL schema, indexes, constraints, or data migrations.
---

# PostgreSQL migration procedure

1. Read the current schema and migration runner before proposing a change.
2. Add a new sequential migration; never rewrite an applied migration.
3. Consider existing rows, locks, table size, constraints, indexes, and rollback/recovery.
4. Keep ownership and integrity constraints in the database where possible.
5. Verify the migration against a disposable PostgreSQL instance and test the application against the resulting schema when available.
6. Document any operational risk and how an operator should recover from a failed deploy.

Do not claim zero-downtime safety without checking the actual DDL and deployment sequence.
