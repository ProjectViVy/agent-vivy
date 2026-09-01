# Acceptance

How a human can tell this worked:

1. **Same-session convenience**: in any session, ask Vivy to view a skill that declares a tool (e.g. `skill_view`), then in the NEXT message ask it to call that tool directly — it works without viewing the skill again. Before this change the second run failed with "tool not selected".
2. **Isolation holds**: in a different session, calling the same tool without viewing its skill still fails — mounts never leak across sessions.
3. **Audit intact**: the Journal still carries one `tool.mounted` event per mount (TT-3); session pin only reads it, nothing new to audit.
4. **Board**: `docs/TODO.md` §0.1 row TT-1 reads `DONE 2026-09-02`; §10 has the closure row. TT-1/1a/2/3/4 are all DONE.
