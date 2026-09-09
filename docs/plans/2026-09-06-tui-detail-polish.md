# TUI Detail-Polish Proposal (Reference: Crush)

- Branch: `feat/tui-detail-polish` (worktree: `../agent-vivy-tui-polish`, branched from `main@c64b63a`)
- Positioning: Fast-iteration branch; merging is optional. Display details only; do not touch the protocol or runtime, and do not introduce new dependencies.
- Scope: `sdk/tui/view` (shared views); if necessary, `sdk/tui/surface` (except for adding read-only display fields; this proposal defaults to **not modifying** surface); do not touch `cmd/vivy/tui.go`.
- Eino capability check: This proposal is entirely about terminal rendering and does not involve the agent loop, model orchestration, or streaming pipeline, so Eino reuse checks do not apply (N/A). Do not add any `eino*` imports; preserve import isolation.

## 1. Current-State Inventory (Avoid Reinventing the Wheel)

The following capabilities **already exist**; explicitly state "do not redo" when assigning work:

| Capability | Location |
|---|---|
| Crush-style three-pane layout (chat / editor / right sidebar), wide/narrow modes + breakpoint | `view/layout.go`, `view/render.go:265-283` |
| Sidebar: title, updated, cwd, host, reasoning, Context (percentage/graded colors/compact hint), Session Usage, Modified Files, MCP/Skills/LSP | `view/render.go:306-540` |
| Rounded Composer box + chips row (model · thinking · mode · provider · context%) + attachment chips + mode-colored border | `view/render.go:748-851` |
| Unified/side-by-side diff, `+N -N` summary row, hunk colors (inside the gate dialog) | `view/render.go:1108-1257` |
| Tool cards: status icons (◉/✔/✖), 8-line truncation + `tui.debug` full-output switch | `view/render.go:664-722` |
| Markdown (glamour, full + quiet versions), md-line cache | `view/markdown.go`, `view/model.go` |
| File completion, command palette, model selector, session dialog, shortcut dialog, gate-approval dialog (including horizontal scrolling and side-by-side diff toggle) | `view/render.go:94-264, 928-1348` |
| Automatic chat following/scrolling (`chatFollow`/`chatScroll`), streaming cursor `▌`, ANSI-safe truncation | `view/render.go:543-658` |
| Bottom chrome row (shortcut hints / err / `run…`) | `view/render.go:812-832` |

Confirmed **gaps** (the source of this proposal): no spinner (repository-wide `sdk/tui` grep for "spinner" returns 0); multiline input displays only the last line (`render.go:755-758`); no empty-input placeholder; no paste protection; tool cards cannot expand; no scroll indicator; no window title; `+%d -%d` in modified files has no colors (`render.go:444`); busy state has only the static text `run…`.

## 2. Feature List and Assignment Batches

`render.go` / `model.go` are hotspot files; proceed serially by "batch" (features in the same batch share an area and should be handled by one subagent or one human lane), and parallelize batches only when they do not overlap.

| Batch | Features | Theme | Main Files | Size |
|---|---|---|---|---|
| A | F2 F3 F4 F11 | Editor (composer) | `layout.go` + `render.go` editor area + `model.go` input handling | ~300 lines |
| B | F1 F12 F6 | Status row/busy state/scroll indicator | `render.go` chrome area + `model.go` tick | ~220 lines |
| C | F5 F9 F13 | Chat body (tool cards/reasoning/empty state) | `render.go` message area + `model.go` key handling | ~260 lines |
| D | F10 F7 | Sidebar colors + window title | `render.go` sidebar area + `model.go` | ~80 lines |

Recommended order: A → B → C → D; submit each feature separately (`feat(tui): ...`) to follow the commit-one-concern rule.

> **Delivery Status (wrap-up 2026-09-07)**: This branch (`feat/tui-detail-polish`, worktree
> `agent-vivy-tui-polish`) delivered all of batch A (F2 `73aa834`, F3 `7f60072`,
> F4 `b93af3e`, F11 `02781d8`) and all of batch B (F1 `8dbf8ff`, F12 `15c7950`,
> F6 `a345a51`). During execution, another lane delivered the same proposal's F1/F12
> (`e2ad7f2`), F5/F9/F13 (`9052ca5`, merge `b108f0c`), F10 (`2768390`), and
> F7 (`83d242e` + audit fix `bb5e294`) in parallel on main. The F1/F12 versions on
> this branch and main are parallel implementations (kept on the branch and not merged;
> if landed, use the main versions and port only F6 and batch A); F6 and batch A have no
> corresponding implementation on main and are unique to this branch. Batches C/D are
> not reimplemented on this branch.

---

## Batch A: Editor Details (Corresponding to the Crush Composer)

### F2 Full Multiline Input Visibility + Adaptive Editor Height (Completed 73aa834)

**Current state**: `renderEditor` (`render.go:748`) uses `strings.LastIndex(display, "\n")` to render only the **last line**, so earlier input lines disappear from the interface; the editor height is the constant `editorHeight = 4` (`layout.go:24`), independent of the content.

**Target display details** (aligned with Crush textarea behavior):
1. Render **all** input lines inside the Composer, split on `\n`; ANSI-safely truncate each line to the content width (`truncate`/`ansi.Truncate`).
2. Grow the editor with the number of lines: `reserve = border (2) + chips row (1) + input lines`, capped at `maxEditorLines = 6`; after the cap, keep the **cursor line** visible (an internal scrolling window, with `…` at the line start to indicate top/bottom truncation, following Crush's "… N more" notation).
3. Keep the cursor `█` fixed at the end of the **last line** (input is always appended at the end, consistent with the existing `m.input` model).
4. In `layout.go`, change `editorReserve(hasAttachments bool)` to `editorReserve(hasAttachments bool, inputLines int)`; add an `editorLines` argument to `layout.computeLayout` or pass it at `renderFrame`; subtract it from `mainH()` accordingly. Compact and wide screens follow the same rules.
5. Empty input always has one line of content height (no need to implement hysteresis that only grows and never shrinks; calculate directly from the line count).

**Implementation Notes**:
- Add `func editorInputLines(input string, width, maxLines int) []string`: split into lines, truncate, and take the trailing window. Put it in `render.go`; the pure function can be tested.
- Have `renderEditor` consume this function; `renderFrame` (`render.go:20`) first calculates `len(strings.Split(m.input,"\n"))` and passes it to the layout.
- Keep the current gate-approval behavior (hidden cursor and unchanged single-line prompt).

**Tests**: Add to `render_test`: single-line/multiline/over-limit lines, lines containing wide CJK characters (validate with `lipgloss.Width`), and lines containing ANSI sequences; in `layout_test`, verify that `mainH` decreases with input-line count and has a lower bound of 1.

### F3 Empty-Input Placeholder (Completed 7f60072)

**Current state**: None. In the empty state, the Composer has only one line, `::: ` + cursor.

**Target display details** (Crush style): When `input == ""` and there is no gate, render a dim placeholder after the prompt, using `p.Dim` (`paletteSubtle`): `Ask something…  / commands · @files · !shell` (short, single-line, truncated to fit). Never show it when there is a draft, a gate is pending, or the sidebar has focus. The placeholder does not count as input, and the Esc-clear logic remains unchanged.

**Implementation Notes**: Add a four-line if block inside `renderEditor`; extract the placeholder text into the `composerPlaceholder` constant.

**Tests**: Empty input includes the placeholder; non-empty input does not; it is hidden during a gate.

### F4 Paste Protection and Large-Paste Notice (Completed b93af3e)

**Current state**: Pasted text goes directly into `m.input` (`tea.KeyRunes`) with no notice; a very large paste can blow up the Composer and is easy to send accidentally.

**Target display details** (aligned with Crush's large-paste guard):
1. When a single input event inserts text `> pasteThreshold` (default 2000 characters or 40 lines, whichever constant is chosen), the content **still enters the draft normally**, but render a warning chip in the Composer's top attachment-row position: `⚠ Large paste · N lines / M characters · confirm before pressing Enter`, reusing `p.PromptWarn` (warning background, high-contrast text).
2. Show the chip only while the draft contains an over-threshold paste segment; it disappears automatically when the user deletes below the threshold (computing from the current total length/line count of `m.input` is sufficient; no need to track the source).
3. Do not implement a collapsed placeholder (that belongs to the model input pipeline); add only a warning chip, preserving the rule that "everything visible in the terminal enters the draft."

**Implementation Notes**: Add the pure function `func pasteGuardChip(input string) string` (returns the chip text above the threshold, otherwise empty); have `renderEditor` append it after the attachments row. Put the threshold constant alongside `layout.go`.

**Tests**: Threshold boundaries (1999/2000 characters, 40/41 lines), and truncation order when the chip coexists with attachment chips.

### F11 Dim the Editor in Busy State (Completed 02781d8)

**Current state**: Border color changes only with the permission mode (`composerBoxStyle`, `render.go:834`).

**Target display details**: When `meta.Busy == true`, switch the border foreground to `p.Dim` (`paletteSubtle`); the send-hint row already has `run…`; restore the mode color when busy ends. As in Crush, the editor is visibly dimmed during execution to indicate that "the current input will be queued." Do not change input availability.

**Implementation Notes**: Add a one-line busy branch to `composerBoxStyle`.

**Tests**: Assert both busy and non-busy border colors (compare `Style.GetForeground()`).

---

## Batch B: Busy State and Status Rows (Corresponding to Crush spinner / status)

### F1 Animated Spinner + Timer (Completed 8dbf8ff; main has a parallel implementation e2ad7f2)

**Current state**: busy has only the static `run…` (`render.go:829`). The TUI has no spinner or elapsed-time display.

**Target display details**:
1. When `meta.Busy` is true, the chrome row displays `⟳ spinner + elapsed time`, such as `⠸ 12s`. Use the braille sequence customary in the bubbletea ecosystem: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, 120 ms/frame.
2. Start timing when busy is first observed to flip from false→true in the current turn (the driver has no start timestamp, so **local monotonic time is sufficient**; display it as a local observation, not as a claimed server-side value—in line with the surface comment "do not infer server-side truth": do not label it "server elapsed time").
3. When busy ends, freeze the last frame and reset on the next frame; `meta.Error` has priority over the spinner (preserve current behavior).
4. Show the spinner in the same way when a gate is pending and submitting (the existing gate.Submitting branch).

**Implementation Notes**:
- In `model.go`, add `spinnerIndex int` and `busyStartedAt time.Time` (the zero value means timing has not started). `busyTimerDone chan struct` is not usable; use the Cmd produced by `tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return spinnerTickMsg(t) })`, rescheduling the next tick on each Update while busy and stopping when busy ends.
- Add `type spinnerTickMsg time.Time`; in `Update`, while busy, handle it with `spinnerIndex=(i+1)%len(spinnerFrames)` and schedule the next tick.
- Extract the pure rendering function `func spinnerLabel(frames []string, index int, startedAt time.Time, now time.Time) string`; inject `now` for deterministic tests.

**Tests**: Modulo frame advancement, busy→idle reset, elapsed text (`<1s`, `12s`, `1m05s`), and error priority.

### F12 Status-Row Information Enhancement (Left Session/Right Environment, Including Queue Count) (Completed 15c7950; main has a parallel implementation e2ad7f2)

**Current state**: The chrome row has only help keys + err/run (`renderInputChrome`, `render.go:812`).

**Target display details**: One row with two segments, left-aligned hints + right-aligned metadata (when width is insufficient, drop the right segment first and then truncate the left, matching the existing fallback order for the chips row):
- Right segment: `⏸ N queued` (in `p.Warn` when `meta.Queued>0`) · `host` (in `p.Dim`) · session title (in `p.Dim`, middle-truncated when too long).
- Left segment: existing shortcut/err/spinner content.
- Fill the middle with spaces to the full width (measured with `lipgloss.Width`, CJK-safe).

**Implementation Notes**: Split `renderInputChrome` into the existing `chromeHints`, the new `chromeMeta`, and the pure function `joinChromeRow(left, right, width)`.

**Tests**: Right-segment width fallback order (title→host→queue), over-width truncation, and no display when queued=0.

### F6 Scroll Indicator and Jump-to-Bottom Prompt (Completed a345a51)

**Current state**: When `chatFollow=false`, there is no indication, so users do not know they are suspended in history or how to return to the bottom.

**Target display details** (Crush's jump-to-bottom notation):
1. When `!chatFollow`, append `↓ end Back to bottom` rendered with `p.HelpKey` to the left of the chrome row (as a key hint), or show `↑ History · N more lines below` when `maxScroll-chatScroll` is large.
2. Add the `end` key (and `G`, following vim convention, only when input is empty) to set `chatFollow=true` and `chatScroll=maxScroll`.
3. Automatically set `chatFollow=false` when the user leaves the bottom by pressing ↑/↓/PgUp/PgDn in the chat view (if the existing logic does not do so, fill it in—check `handleChatViewKey`, `model.go:742-760`).

**Implementation Notes**: `renderChat` already computes `maxScroll`/`offset`; pass `(offset, maxScroll, follow)` to chrome or store it in a render-time temporary structure. Avoid global state and assemble the chat hint row with the chrome row directly in `renderFrame` (rework the concatenation order in `renderWide`/`renderCompact`).

**Tests**: Text for the follow/hover states, `end`/`G` keys returning to the bottom, and no display when `maxScroll==0`.

---

## Batch C: Chat-Body Details (Corresponding to Crush Message Rendering)

### F5 Expand/Collapse Tool Cards (Delivered by the main lane in 9052ca5; do not reimplement on this branch)

**Current state**: Tool results are fixed at 8 lines (`compactToolResultLines`, `render.go:664`), and full output requires changing the `tui.debug` configuration and restarting. Crush's tool blocks can be interactively collapsed.

**Target display details**:
1. Change the truncation marker from `… N more lines · set tui.debug: true` to `… N more lines · ctrl+o expand` (retain the English version; this matches the existing mixed-copy style, with English selected).
2. Add the `ctrl+o` key to toggle "expand the most recently completed tool card" (the last card with `status==done|failed|denied`). Press it again to collapse. In the expanded state, render all lines for that card (still wrap to width + ANSI truncate; do not introduce horizontal scrolling).
3. State storage: add `expandedTools map[string]bool` to `Model` (key=`ToolCallID`; if empty, fall back to `fmt.Sprintf("%s#%d", ToolName, index)`). This affects rendering only and not the surface protocol.
4. Expand everything when `debugToolOutput=true` (preserve the existing semantics; change the marker text to `· debug`).
5. Add `▾` to the end of the card title in the expanded state and `▸` in the collapsed state (do not add either to pending cards).

**Implementation Notes**: Add an `expanded bool` parameter to `renderToolWithOptions` (and update `renderMessageWithOptions` and the mdCache decision—the tool card is not cached anyway; confirm that `cacheableMarkdownMessage` already excludes `Tool != nil` in `model.go`); have `Update` handle `ctrl+o` by locating the most recently completed card.

**Tests**: 8-line truncation marker, full expansion, ctrl+o toggle round trip, locating the last completed card among multiple tools, and expansion not affecting mdCache.

### F9 Collapse Reasoning Blocks (Delivered by the main lane in 9052ca5; do not reimplement on this branch)

**Current state**: Reasoning messages are rendered as full Markdown (gray italics), so long reasoning fills the screen; streaming reasoning is also rendered in full.

**Target display details** (aligned with Crush's thinking behavior):
1. **Completed** reasoning blocks (`Reasoning && !Streaming`) are collapsed to one line by default: `┊ ✻ Reasoning complete · N lines · ctrl+r expand`, styled with `p.Reasoning`. Render in full when expanded, then collapse again with the same key.
2. Keep **streaming** reasoning fully expanded (it is being generated, so collapsing it is not meaningful).
3. Add the `expandedReasoning map[string]bool` state, using the same key rule as F5; `ctrl+r` toggles the most recently completed reasoning block.
4. User and assistant messages are unaffected.

**Implementation Notes**: Add a reasoning-collapse branch at the start of `renderMessageWithOptions`; derive `N lines` in the collapsed line from the number of lines produced by `renderMessageBody` before collapsing (render once to count lines, or estimate from content—render directly to count, since mdCache already exists).

**Tests**: Collapsed single line includes the line count, expand/collapse round trip, streaming does not collapse, and ctrl+r targeting.

### F13 Empty-Session Welcome State (Hero) (Delivered by the main lane in 9052ca5; do not reimplement on this branch)

**Current state**: The empty state has 3 lines (`chatLines`, `render.go:564-566`) and is basic. Crush has a centered brand area + hints.

**Target display details**:
1. Render a centered hero for an empty session: a large `VIVY CODE` wordmark (reuse `p.LogoWord`, optionally add a decorative `p.Diagonals` row), one slogan line `A journey toward a true heart`, then a set of dim hint rows (each with a `p.HelpKey` key + `p.HelpDesc` description): `/ command palette`, `@ file references`, `shift+tab switch mode`, `! shell`, `ctrl+s sessions`.
2. Center it approximately within the visible area vertically (starting around 1/3 of the chat height); do not add scrolling behavior.
3. Remove it immediately once messages exist (the existing `len(messages)==0` branch already handles this).

**Implementation Notes**: Extract `func renderEmptyState(width, height int, p Palette) []string`; use lipgloss `Align(lipgloss.Center)` + `Height`.

**Tests**: Line count does not exceed the visible area, narrow widths do not fail (80/60/40-column snapshots assert keyword inclusion), and it is not rendered when messages exist.

---

## Batch D: Sidebar and Title (Small Changes, Wrap-Up)

### F10 Sidebar Modified Files Colors + Path Truncation (Delivered by the main lane in 2768390; do not reimplement on this branch)

**Current state**: `render.go:444` directly uses `fmt.Sprintf(" %s  +%d -%d", ...)`, so additions and deletions have no color; long paths rely on outer truncation that hard-cuts the end (and can remove the extension).

**Target display details**:
1. Use `p.DiffAdd` for `+N` and `p.DiffDel` for `-N` (the same semantic colors as the diff body).
2. When a path is too long, **preserve the head and tail** (`very/long/…/path/file.go`, with a middle ellipsis) so the filename is always visible; reuse or create the pure function `truncateMiddle(path, width)` (account for CJK/wide characters and measure with `lipgloss.Width`).
3. Do not add hover highlighting to file rows (there is no mouse semantics; keep keyboard priority).

**Tests**: Short paths remain unchanged, long paths are middle-truncated with an ellipsis and correct total width, and color styles are asserted.

### F7 Terminal Window Title (Delivered by the main lane in 83d242e + audit fix bb5e294; do not reimplement on this branch)

**Current state**: It is not set (after `view.Run` starts altscreen, the title is `vivy`/the process-argument semantics passed by `--title TUI`).

**Target display details**: After a session switch/rename, call `tea.SetWindowTitle(title + " · VIVY CODE")`; fall back to `VIVY CODE` for an empty title. Display-only, with no protocol impact.

**Implementation Notes**: In `Update`, send `tea.SetWindowTitle` (returning a tea.Cmd) while handling `surface.SessionsMsg` (rename/list) and the session-switch path. Note that `SetWindowTitle` is a `tea.Cmd` and cannot be called from View.

**Tests**: Testability is limited; unit-test the pure title-composition function `sessionWindowTitle(title string) string` (including empty-title fallback and overlong truncation).

---

## 3. Assignment Template (Append This Common Header to Each Subagent Prompt)

> Working directory: `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-tui-polish` (git worktree, branch `feat/tui-detail-polish`). Do not modify the root worktree at `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`. Implement only the features listed in this assignment; do not make opportunistic changes. One commit per feature, with commit message `feat(tui): <topic>`, staging only that feature's files. Definition of done: `go build ./...` + `go test ./sdk/tui/...` all green + new tests covering the listed paths + update the corresponding feature entry in this proposal to "completed (commit hash)". Visual self-check: copy `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy\sdk\tui\view\zpreview_test.go` to the same path in the worktree (untracked, do not commit), adapt preview cases as needed, and manually inspect the output of `TUI_PREVIEW=1 go test ./sdk/tui/view -run TestDump -v`.

Within a batch, have **one subagent handle multiple features sequentially** (render.go/model.go have a large conflict surface, so do not parallelize); different subagents may work on separate batches in parallel only if they strictly touch their own "main files" lists; otherwise proceed serially.

## 4. Verification and Delivery

- Each batch: `go build ./...`, `go test ./sdk/tui/...`; run an additional `TUI_PREVIEW=1` manual check for batches A/B.
- Wrap-up: run `just ci` (inside the worktree; kernel/UI gates are unchanged; use the `sdk/tui` result if covered, or record the reason in verification.md if not—if `just ci` does not cover `sdk/`, add and note a `go test ./sdk/...` line).
- Delivery log: `docs/logs/2026-09-06-tui-detail-polish/` (summary.md / verification.md / acceptance.md); complete it once at wrap-up, listing the commit for each feature.
- `docs/TODO.md` §0.1: register issues discovered during the work but not fixed on this branch according to the rules.
- Real-device smoke test: after connecting with `just run` + `vivy tui --live`, manually walk through the visible behavior of all 12 features and record it in acceptance.md.

## 5. Explicitly Out of Scope

- Do not change `surface` protocol fields or add RPC (F8 requires protocol additions for message-level token/cost, which exceeds "display details only"; do not do it, register it in TODO).
- Do not introduce new dependencies (implement the spinner with `tea.Tick` rather than bubbles; if the assigned agent believes bubbles' textarea/spinner is substantially better, stop and report first, and do not add the dependency independently).
- Do not add mouse support or a theme system (palette is already the single source); do not change build paths outside packed face.
