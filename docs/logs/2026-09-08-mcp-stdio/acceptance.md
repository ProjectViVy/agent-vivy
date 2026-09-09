# Acceptance

1. On the MCP settings page, add a service, select `STDIO`, and enter a PATH command or absolute path; the parameter text box takes one argv per line, each environment mapping is `CHILD_VAR ← HOST_VAR`, and cwd may only be a relative path under the workspace root.
2. After saving, reopen the settings page; the stdio configuration, environment-name mapping, and cwd should all be echoed back. Exporting JSON and importing it again preserves these fields and never exports environment values.
3. Enabling a stdio service does not launch a process immediately; the first probe/list/call starts it exactly once. When a host environment variable is missing, the service remains unstarted and shows `error`; after the environment is supplied, configuration replacement/reload must recover it.
4. After a stdio child process exits, the next operation shows `error` and does not implicitly start a second process. Existing HTTP-service behavior remains unchanged.
5. TUI/sidebar continues to use the same sidebar route and configured/initialized/error state contract; the added transport/env_missing fields are additive only, and the stdio type, missing child key, and death error are all visible.

Acceptance boundary: the raw-frame upper bound and Windows process-tree cleanup must be closed by a future TODO before complete local-process governance can be claimed.
