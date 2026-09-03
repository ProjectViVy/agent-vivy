# Summary

This delivery adds a real, bounded, read-only MCP resources surface to VIVY
CODE. The shared command parser accepts `/mcp resources <server>` and
`/mcp read <server> <uri>` while retaining `/mcp` catalog and `/mcp <server>`
one-shot probe behavior.

The runtime backend speaks MCP `resources/list` and `resources/read` over the
existing Streamable HTTP JSON/SSE transport. It follows opaque pagination
cursors within a page cap and shared byte budget, matches JSON-RPC response
IDs across multi-event SSE, and sends the negotiated protocol-version header.
Resource identity and metadata
(`server`, `uri`, `name`, `title`, `description`, `mime_type`, `size`, and
metadata fields) plus text/blob content are projected through new read-only
control RPC routes. Config-file servers and settings-overlay servers resolve
through one effective catalog, while an explicit empty overlay remains
authoritative. Remote output is bounded and explicitly marked
`untrusted`; no resource is mounted, written to tenant data, or forwarded to
the model. Empty text/blob representations remain distinguishable, remote MCP
error codes retain useful NotFound/InvalidParams semantics, and terminal-bound
error text is redacted and stripped of control characters.

The built-in fullscreen TUI, packed `faces/tui` face, and legacy REPL share the
same validation, routing, and conservative result labels. Prompts and tools
remain unchanged and no MCP side-effect operation is added.

Not done: MCP stdio/OAuth, resource change notifications, prompts, and
model-visible resource tools remain outside this slice.
