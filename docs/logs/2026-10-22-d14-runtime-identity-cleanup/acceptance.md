# Scoped acceptance observations

The regression cases express the following observable results:

1. Replaying a steering delivery does not add a duplicate user message. A
   changed same-length steer reaches the same history while pending tool calls
   remain at the tail.
2. Repeated model selection reuses its client; same-length credential rotation
   changes outbound authorization, and an unready selection cannot use a cached
   client to contact the provider.
3. A selected workspace reopens with the same session-owned ID. A separate
   session retains its own ID even at the same root. Inside-root paths pass,
   outside paths fail, and release cannot remove the selected project.
4. A same-length tool result edit and in-place attachment/file metadata edits
   appear in the TUI. A paused viewport remains anchored while earlier content
   grows; repeated unchanged refreshes do not reapply its offset.
5. MCP prompts differing in case or slug punctuation remain distinct in
   `commands/list` and expand to their original exact server/prompt pair.
   Maximum-sized valid identities remain accepted by the TUI registry.
6. Current redacted shell previews survive history projection. Ordinary raw
   shell arguments and malformed preview strings remain absent from previews.

These are native in-process owner checks. A real split-browser/terminal smoke,
Linux candidate gates, and copied-store conversion acceptance belong to the
integrated candidate and are not certified by this patch.
