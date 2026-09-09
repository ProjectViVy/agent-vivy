# Lane C chat body: F5 tool-card ctrl+o expansion · F9 reasoning ctrl+r collapse · F13 empty-session hero

## Delivered

All changes are in the shared presentation layer `sdk/tui/view`; `vivy-code.exe`, `vivy tui` (including `--live`), and the packaged TUI face paths all take effect in sync.

- **F5 tool-card ctrl+o expansion**: Added the session-scoped `toolExpanded` toggle (`KeyCtrlO`, ignored when a gate exists); it is active through the `tui.debug` configuration or temporary ctrl+o expansion. The `compactToolLines` omission marker is now `… N more lines · ctrl+o expand`. Tool-card rendering does not enter mdCache, so there is no cache-invalidation issue; the meaning of the `tui.debug` configuration is unchanged.
- **F9 reasoning ctrl+r collapse**: Added the session-scoped `reasoningCollapsed` toggle (`KeyCtrlR`, ignored when a gate exists); when collapsed, reasoning messages render as the one-line summary `reasoning · N lines · ctrl+r expand` (retaining the ReasoningBar/Reasoning styles and chips logic), and expand to their original form. `messageMarkdownKey` and the cache `ensure` gain a `collapsed` dimension, ensuring that a toggle at the same width does not read stale rendering.
- **F13 empty-session hero**: The empty `chatLines` branch is upgraded from a 3-line placeholder to a hero (`Vivy™ VIVY CODE` wordmark, “Journey to Find Your True Heart”, the CWD line when populated, and command/key hints); narrow widths use the existing `truncate`; non-empty sessions do not render it.
- The shortcut panel (`ctrl+x`) adds `ctrl+o tool output` and `ctrl+r reasoning` as two lines.

## Explicitly not done

- Did not change the surface/domain contract, driver layer, Journal, provider, or Eino orchestration (presentation-only; no Eino surface applies).
- Did not implement per-card targeted expansion/collapse (ctrl+o/ctrl+r are session-wide toggles); toggle state is not persisted (each startup returns to the default).
- Did not handle hard truncation of the hero wordmark at ultra-narrow widths (`padBlock` fallback, cosmetic-level, outside this round's scope).
- The two untracked scratch files at the repository root (`sdk/tui/view/zpreview_test.go`, `tui-composer-shot.png`) belong to the previous lane and are not included in this delivery.

## Process

Independent worktree `feat/tui-chat-body-polish` (VC track's parallel-isolation rule). Sub-agents split the work: the builder implemented F5/F9/F13 and tests, and the reviewer audited the diff (SHIP; three P2 items: key labels were changed to neutral wording and tests removed assertions on unexported fields; both were fixed, and narrow-width hero truncation was recorded as not done). The supervisor handled integration, tests, and `just ci`.
