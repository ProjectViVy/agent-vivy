# Acceptance — how a human can tell it worked

Run `vivy-code` in a project.

1. Fresh launch: the rail shows `untitled session` and the terminal title is
   the bare `VIVY CODE` — sessions no longer inherit a placeholder name.
2. Send any message. When the run ends, the session names itself within a
   few seconds: an LLM-generated short title (when the provider is
   reachable) appears in the rail header, the sessions dialog (ctrl+s), and
   the terminal title becomes `VIVY CODE · <title>`.
3. If the first run fails (bad tool call, provider error), the session is
   still named — the title falls back to the first few characters of the
   user's message.
4. `/rename` keeps winning: a user-chosen title is never overwritten, and
   `/new` without arguments produces another untitled session that will
   name itself after its own first exchange.
