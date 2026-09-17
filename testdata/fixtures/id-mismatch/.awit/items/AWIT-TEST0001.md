---
id: AWIT-TEST0009
title: Implement OAuth2 bearer token extraction
brief: Fix header parsing so URL-safe bearer tokens authenticate.
status: open
deps: []
labels: [auth, p1]
refs: []
---

## Summary

The id key does not equal the filename stem.

## Acceptance Criteria

- LoadAll reports ID MISMATCH; Broken.ID is the stem AWIT-TEST0001
