The Authorization header is Bearer followed by a single token. The token
uses the URL-safe base64 alphabet and is verified against the signing key.

A failed verification returns HTTP 401 with a JSON body containing error
invalid_token. The gateway never returns 500 for a bad header.
