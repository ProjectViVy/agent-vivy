# Acceptance — TUI-MD-TOOL-RESULTS

How a human can tell it worked (run `vivy-code.exe` or `vivy.exe` TUI):

1. Trigger any tool whose result is JSON (for example an RPC/HTTP tool
   returning an object). The tool card body now shows pretty-printed,
   syntax-coloured JSON instead of a flat wrapped string.
2. Trigger a file patch/edit tool. The card body is still diff-coloured
   (green/red as before) — diff detection did not regress.
3. Trigger a tool that returns a markdown report (headings, `- ` lists).
   The card body renders with heading emphasis and coloured list markers
   instead of raw `##`/`-` characters.
4. Trigger a tool that returns plain text (a `dir`/`ls` listing). Output is
   unchanged from before — plain wrapped lines, no code-block background.
5. Long results still collapse to 8 lines with the `… N more lines ·
   ctrl+o expand` hint, and `ctrl+o` still expands the full body.

Rollback: revert the single commit for this slice; `renderToolWithOptions`
returns to plain `wrapText` bodies.
