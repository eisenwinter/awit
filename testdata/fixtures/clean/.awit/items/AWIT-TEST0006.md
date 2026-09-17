---
id: AWIT-TEST0006
title: Implement refresh-token rotation
brief: Issue a new refresh token on every use and revoke the previous one.
status: in_progress
deps: [AWIT-TEST0005]
labels: []
assignee: agent/claude
claimed_at: 2026-09-17T14:32:05Z
refs: []
---

## Summary

Reuse of a stolen refresh token currently goes undetected.

## Acceptance Criteria

- Each refresh call mints a new token and invalidates the old one
- Concurrent reuse of the old token is rejected
