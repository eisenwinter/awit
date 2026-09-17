---
id: AWIT-TEST0004
title: Rewrite the parser in place
brief: Replace the extractor without a staging flag, then revert if it fails.
status: open
deps: [AWIT-TEST0004]
labels: []
refs: []
---

## Summary

Self-loop used by SCC tests. The item depends on itself.

## Acceptance Criteria

- File parses as an open item whose deps list contains only its own ID
