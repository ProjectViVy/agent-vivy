# G2 — Configurable keybindings

Story: `docs/superpowers/plans/vivy-code-parity/G2-keybindings.md`
Commit: `feat(tui): configurable keybindings`

## What shipped

- New `sdk/tui/view/keymap.go`: `Keymap` = ordered action→chords table +
  chord→action index. 24 named actions cover every global chord previously
  hardcoded in `handleKey` (quit, palette, shortcuts, sessions, new_session,
  model_picker, model_cycle, permission_cycle, thinking_cycle, tools_toggle,
  reasoning_toggle, mode_cycle, sidebar_focus, dequeue, follow_up, send,
  newline, cancel, fast_quit, jump_bottom, page_up, page_down, top, bottom).
- `LoadKeymap(path)` reads `keybindings.yaml` (`action: chord` or
  `action: [chords]`). Missing file → defaults silently; invalid YAML or
  unreadable file → defaults + startup warning; unknown action → warning;
  chord claimed by two actions → the earlier-declared action wins
  deterministically + warning. Overrides *replace* an action's chord list.
- Chord normalization: modifiers/named keys case-fold; a bare rune keeps
  case ("G" ≠ "g") while a modified rune folds ("alt+P" = "alt+p"),
  matching bubbletea `KeyMsg.String()`.
- `handleKey` now routes every global key through `m.keys.Action(keyChord)`
  → `runBoundAction`, which reproduces the previous per-key guards
  (gate/busy/queued/input-empty) verbatim. Actions whose guard fails return
  `handled=false` so printable chords ("q", "G", "H", "/") still type when
  the composer holds a draft. Remapping `send` off enter makes enter act as
  a newline outside gates.
- In-dialog keys (approval diff scroll, sessions/palette/tree navigation,
  gate y/n) stay fixed by design — they are per-surface conventions, not
  global chords; the submitting-gate path honors a remapped `quit`.
- `/hotkeys` command (registry + validation + en/zh descriptions + dialog
  title) prints the effective binding table.
- Wiring: `faceport.Options.KeybindingsFile` → `view.Options.KeybindingsFile`;
  `<shared settings dir>/keybindings.yaml` for vivy-code,
  `<data dir>/keybindings.yaml` for `vivy`/`vivy tui`. Load warnings join
  the startup overlay alongside theme warnings.

## Notes

- `faces/tui` is a separate Go module: adding `gopkg.in/yaml.v3` to
  `sdk/tui/view` required `go mod tidy` there, which moved the module's
  source digest. Pinned chain updated: `faces/tui/{face.go,vivy-module.yaml}`,
  `reproduction_test.go` table entry, `conformance_results.json` —
  all to `9b27d803…` (self-referential fixed point, same as other modules).
- Internal digest re-pinned to `cd8d7bca…` (internal/codeface changed).
