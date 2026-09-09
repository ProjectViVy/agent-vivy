# acceptance — FACE-TUI-1 F2

## How a human verifies it

1. **Built-in behavior is unchanged**: without repacking with vivy-sdk, run the usual
   `vivy.exe run "…"` directly — behavior is exactly the same as before F1 (the kernel
   headless loop). The default registry is nil; the face branch activates only in a
   generation packed with `--face`.
2. **Five steps to produce a "mouth-equipped" generation**:
   - `vivy-sdk verify faces/headless` → ok
   - `vivy-sdk pack --face headless` → new EXE + generation.json, where `recipe.face` =
     `headless`, `face.kind` = `headless`, and grants contain only tty/argv/rpc.client
   - Run `vivy.exe run --continue "x"` beside that EXE (empty log) → stderr shows
     `headless: no sessions to continue…` (the organ is speaking, not the kernel)
3. **§14④ is visible**: trigger a tool requiring approval against the packed EXE (or see
   the unit test `TestApprovalBlockCancelsAndReturnsCancelled`) → stderr prints
   "requires human approval and the headless face cannot ask for it"; the run is cancelled
   (exit 2), and the process neither hangs nor silently allows it.
4. **SDK gatekeeping**: corrupt the `faces/headless` manifest (for example, add tools,
   set `listen: true`, or add fs.read to Grants) → `vivy-sdk verify` reports an error and
   refuses to pack.

## Boundaries (explicitly not accepted here)

- Interactive TUI in faces/tui, faces/web migration, and `face.listen: true` — outside this
  slice.
- Per-grant runtime arbitration for face grants (the verifier gates this batch).
