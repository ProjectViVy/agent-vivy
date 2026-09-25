# VIVY CODE project initialization

The VIVY CODE terminal now offers `/init`. It inspects the current code project through the ordinary governed model turn, then creates a concise `AGENTS.md` when none exists. If `AGENTS.md` exists, the turn runs in Plan mode and suggests changes for the user's approval without editing files. The draft run mode and pending image attachments are preserved.

The control plane checks the authoritative code root for `AGENTS.md` before dispatch and fails closed on an inaccessible or nonregular entry. The generic Vivy terminal does not advertise `/init` without that project capability. The startup instruction scan reserves a missing launch-directory `AGENTS.md` so a file created during a session is loaded on the next turn, even when ancestor rules already exist.

The offline `vivy init` starter command, browser UI, and Studio are outside this change.

## Eino capability check

The pinned `github.com/cloudwego/eino/adk/middlewares/agentsmd` already reads its configured files for each run. Vivy uses that middleware and its project backend; no new instruction injector or agent loop is needed. Eino does not own terminal slash commands or the authoritative project-file presence check, so the small read-only RPC method handles that gap. Existing runtime Plan policy denies effectful tools for the suggestion turn. If the middleware gains dynamic path discovery, the launch-directory placeholder can be removed.
