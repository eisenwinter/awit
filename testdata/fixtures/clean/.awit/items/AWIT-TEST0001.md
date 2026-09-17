---
id: AWIT-TEST0001
title: Implement OAuth2 bearer token extraction
brief: Fix header parsing so URL-safe bearer tokens authenticate.
status: open
deps: []
labels: [auth, p1]
refs: []
---

## Summary

The auth middleware splits on spaces and assumes the token is strict base64.
URL-safe tokens with `-` and `_` are dropped before signature verification.

## Acceptance Criteria

- Tokens using the URL-safe base64 alphabet authenticate successfully
- Invalid signatures still return a structured 401
