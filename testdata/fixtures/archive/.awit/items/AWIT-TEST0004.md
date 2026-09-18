---
id: AWIT-TEST0004
title: Fix URL-safe base64 token rejection
brief: The middleware rejects URL-safe bearer tokens before signature checks.
status: closed
deps: []
labels: []
refs:
  - ../comments/AWIT-TEST0004/20260916T090000Z-jan.md
  - ../comments/AWIT-TEST0004/20260916T091500Z-jan.log
  - ../comments/AWIT-TEST0004/20260916T093000Z-claude.md
---

## Summary

URL-safe base64 characters in bearer tokens must authenticate.

## Acceptance Criteria

- Tokens from the URL-safe alphabet authenticate successfully
