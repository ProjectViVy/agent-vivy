# Acceptance

1. In either fullscreen code face or the legacy REPL, `/mcp` still shows the
   configured catalog and `/mcp <server>` still performs the one-shot tools
   probe.
2. `/mcp resources docs` calls the control-plane resources/list route and
   displays the server, URI, names, descriptions, MIME type, and an explicit
   untrusted/read-only label.
3. `/mcp read docs "docs://guide"` calls resources/read and displays text or
   base64 blob content without mounting or writing it locally.
4. A configured server returning either JSON or SSE MCP responses works; an
   unknown server, malformed response, oversized response, or oversized
   content fails or truncates within the documented bounds while retaining a
   useful cause.
5. No `/mcp` resource command starts a model turn, executes a remote tool, or
   writes `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
