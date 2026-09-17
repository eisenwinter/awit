---
id: AWIT-TEST0003
title: Add E2E auth tests
brief: Cover login, refresh, and 401 paths against a running gateway.
status: open
deps: [AWIT-TEST0001]
labels: []
refs: []
---

## Summary

Happy-path login is untested. Failures currently show up only in staging.

## Acceptance Criteria

- Login, refresh, and invalid-signature cases are asserted
- Tests run in CI without a live network
