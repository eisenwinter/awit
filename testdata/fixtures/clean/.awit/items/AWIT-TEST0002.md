---
id: AWIT-TEST0002
title: Update database migration scripts
brief: Add the missing users.token_hash column and backfill existing rows.
status: open
deps: []
labels: [db]
refs: []
---

## Summary

The token_hash column is referenced by the auth service but never created.

## Acceptance Criteria

- Migration adds users.token_hash
- Existing rows are backfilled without downtime
