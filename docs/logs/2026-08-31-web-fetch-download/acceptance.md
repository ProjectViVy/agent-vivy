# Acceptance — web_fetch + download (2026-08-31)

## How a human can confirm it works

Prerequisite: `config.yaml` needs no new configuration (both tools are enabled by default).

1. **Ask Vivy to fetch a webpage** (in a session): "Use web_fetch to fetch https://example.com"
   → the response should contain the "Example Domain" body (markdown format), rather than
   "host not allowlisted". This is the boundary with `http_request`:
   `http_request` can fetch only hosts in the `http_allowed_hosts` allowlist, while `web_fetch`
   can fetch any public webpage.
2. **Format control**: "Use web_fetch to fetch the xxx page, format=text" → returns plain text with collapsed whitespace;
   `format=html` → returns the HTML inside the body.
3. **Security boundaries are visible**:
   - "Use web_fetch to fetch http://localhost:8787/" → rejected (private or local),
     unlike `http_request` (which permits allowlisted localhost).
   - A URL with `?token=...` → rejected.
   - A direct binary link (such as a zip) → prompts you to use download.
4. **download uses approval**: "Download https://example.com as example.html"
   → the familiar approval card appears (with the URL, output path, and the "writes remote content into
   the workspace" risk notice); after approval the file appears in that run's workspace;
   downloading the same target again adds "overwrites existing file" to the card. Rejection writes nothing.
5. **Discoverable through tool_search**: ask "What network tools do I have?" in a new session → tool_search
   shows web_fetch / download in its index.

## Differences from old behavior (user-visible)

- Before: arbitrary public webpages could not be fetched (`http_request` was blocked by its allowlist),
  and "read a webpage into context" required a network_search summary.
- Now: one sentence fetches any public webpage into context (markdown first), and files can be written to disk with approval.
- `http_request` capabilities and security semantics are unchanged; existing users are unaffected.

## What is not being accepted

- agentic_fetch / sourcegraph are outside this iteration (see TODO §0.1 WEB-1).
- The model-side accuracy of automatic tool selection (model-dependent; the tool description includes the
  mutually exclusive hint "For reading content into the conversation use web_fetch").
