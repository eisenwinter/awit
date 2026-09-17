---
id: AWIT-TEST0004
title: Rotate API tokens
brief: Re-issue production API tokens after the extractor fix and E2E coverage.
status: open
deps: [AWIT-TEST0001, AWIT-TEST0003]
labels: [p0]
refs: []
---

## Summary

Tokens minted under the old extractor cannot be distinguished from forgeries
once the parser is fixed. Rotation is the cutover.

## Acceptance Criteria

- Every production token is re-issued
- Old tokens are rejected within one TTL
