# VCP — VIVY CODE Pi Parity Story Index

Revision VCP-P1, 2026-10-06. Initiative: close the pi-agent (`earendil-works/pi` @ `23cf2b9`) capability gap on agent-vivy `main` baseline `f34f3ce`. Spec: [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md). Gap research: `docs/research/` entry pending — the two authored artifacts live at session attachments (v1 gap matrix; v2 decision-adjusted closure plan) and will be committed with this package.

## Authority and scope

- Owner decisions VCP-O1..O8 are binding (spec §2). No runtime extension mechanism, no non-Anthropic/OpenAI wire protocols, no `/share` upload, no codemode, no self-update, no CLAUDE.md chain, no mermaid, no remote-exec env.
- Only `internal/runtime` and `internal/provider` may import Eino. Kernel deltas (steering, compaction signature, session tree storage, thinking levels, cache warming, face options) are in-scope kernel work, not plugin work.
- AI identity never authors commits/PRs; every commit carries the human identity via per-command env vars.
- Each Story ends in focused commit(s), an iteration log under `docs/logs/YYYY-MM-DD-<slug>/`, and `just ci` (or recorded environment blockers).

## Requirements → Epics

| Req | Epic |
|---|---|
| RQ-CLI, RQ-JSON, RQ-RPC, RQ-SDK, RQ-SUB | A — CLI/embedded surface |
| RQ-STEER | B — steering + queue |
| RQ-SESS | C — session tree + portability |
| RQ-CMP (+RQ-GUI compact) | D — compaction parity |
| RQ-TOOL | E — tool plane |
| RQ-MDL | F — model host |
| RQ-TUI | G — TUI surface |
| RQ-LLM | I — local-LLM compat plugin |
| RQ-BND | H — bundle consolidation |

## Story ledger

| Story | Epic | Requirements | Immediate predecessors (supplies) | Outcome | Status |
|---|---|---|---|---|---|
| [A1](A1-flags-face-options.md) | A | RQ-CLI | None | Flag parser + extended `face.Options` + config plumbing | Ready |
| [A2](A2-print-json-modes.md) | A | RQ-CLI, RQ-JSON | A1 (options.Mode, face runner seam) | `--mode print|json` headless output | Ready after A1 |
| [A3](A3-rpc-mode.md) | A | RQ-RPC | A1; B1 for steer/follow_up/queue cmds; C1 for tree/clone/export cmds | `--mode rpc` JSONL command shim | Blocked by B1,C1 command deps (partial impl OK earlier) |
| [A4](A4-codeclient.md) | A | RQ-SDK | A3 | `sdk/codeclient` Go client | Blocked by A3 |
| [A5](A5-subcommands.md) | A | RQ-SUB | A1 | `vivy-code mcp|config` subcommands | Ready after A1 |
| [B1](B1-kernel-steering.md) | B | RQ-STEER | None | Kernel dual-track queue + steer injection + `turn/steer|follow_up|queue/clear|queue/mode` RPC + Journal events | Ready |
| [B2](B2-tui-steering.md) | B | RQ-STEER | B1 | TUI keys Enter/Alt+Enter/Alt+Up + queue display | Blocked by B1 |
| [B3](B3-gui-steering.md) | B | RQ-STEER, RQ-GUI | B1 | GUI steer/follow-up split | Blocked by B1 |
| [C1](C1-session-tree-storage.md) | C | RQ-SESS | None | `session/tree|clone|import|export` storage + RPC | Ready |
| [C2](C2-tui-session-cmds.md) | C | RQ-SESS | C1 | `/tree /clone /import /export /copy /bug /debug` | Blocked by C1 |
| [C3](C3-gui-session.md) | C | RQ-SESS, RQ-GUI | C1 | GUI tree page + export download | Blocked by C1 |
| [D1](D1-compaction-instructions.md) | D | RQ-CMP | None | `CompactSession(instructions)` + per-model overrides + file tracking + RPC/TUI/GUI wiring | Ready |
| [D2](D2-overflow-recovery.md) | D | RQ-CMP | D1 | overflow/length → compact-and-retry once | Blocked by D1 |
| [D3](D3-gui-compact.md) | D | RQ-CMP, RQ-GUI | D1 | chat-level compact control (folded into D1 scope if trivial — see plan) | Blocked by D1 |
| [E1](E1-shell-tool-upgrade.md) | E | RQ-TOOL | None | env injection, shell prefix, `!!`, output-to-file | Ready |
| [E2](E2-tool-exposure.md) | E | RQ-TOOL | None | `ToolSpec.Exposure` + model-visible filtering + `tools/activate` | Ready |
| [E3](E3-tool-search.md) | E | RQ-TOOL | E2 | `tool_search` module (coding-tools) | Blocked by E2 |
| [F1](F1-thinking-levels.md) | F | RQ-MDL | None | 7-level thinking + per-model clamp/defaults + sampling map | Ready |
| [F2](F2-cache-warming.md) | F | RQ-MDL | None | cache warming scheduler (off|streaming|idle) | Ready |
| [F3](F3-model-ux.md) | F | RQ-MDL | F1 | model cycle + scoped models + save-default | Blocked by F1 |
| [G1](G1-themes.md) | G | RQ-TUI | None | JSON themes + auto detect | Ready |
| [G2](G2-keybindings.md) | G | RQ-TUI | None | `keybindings.yaml` named actions | Ready |
| [G3](G3-tui-extras.md) | G | RQ-TUI | None | search/prompt-jump/copy/ext-editor/startup listing/OSC8 + tool renderer registry | Ready |
| [G4](G4-inline-images.md) | G | RQ-TUI | None | kitty/iTerm2 inline images, flag-gated | Ready (risky, may descope) |
| [I1](I1-local-llm.md) | I | RQ-LLM | None | `coding-local-llm` module | Ready |
| [H1](H1-bundle-consolidation.md) | H | RQ-BND | E2,E3; C1; A2,A3 | `plugins/coding/*` + recipe finalization + Inspect evidence | Blocked by earlier Epics |

## DAG and waves

```text
W1 (kernel/face foundations, independent):
    A1 ──┬─► A2 ─► A3 ─► A4
         └─► A5
    B1 ─► B2, B3
    C1 ─► C2, C3
    D1 ─► D2, D3
    E1, E2 ─► E3
    F1 ─► F3
    F2
    G1, G2, G4
    I1
W-last:  H1 (after E3, C1, A2/A3 exist)
```

Dependencies are logical (contracts), not calendar order. Kernel stories (B1, C1, D1, E2, F1, F2) land first in practice because faces consume their RPC surface.

## Shared-file scheduling

| Boundary | Owners | Rule |
|---|---|---|
| `internal/runtime/service.go` + consume loop | B1, D1, D2 | One lane; D-stories rebase on B1's boundary hooks |
| `internal/rpc/control.go` dispatch | B1, C1, D1, E2, A3-shim consumers | Sequential edits; each story registers only its methods |
| `sdk/port/face/face.go` Options | A1 | Frozen after A1 lands; modes consume it |
| `sdk/tui/command` registry | B2, C2, D3, F3, G3 | Sequential; each adds only its Specs |
| `sdk/tui/view` render/styles | G1, G2, G3, G4, B2 | Sequential face edits |
| `internal/provider` + `internal/modelhost` | F1, F2 | One model-host lane |
| `internal/tools` (protected) | E1 | T1 only |
| `internal/config` schema | A1, E1, E2, F1, F2, I1 | Sequential; additive fields only |
| `internal/storage` migrations | C1 | Owns next free migration number at its execution time |
| `recipes/vivy-code.vivy.yml` | A1 (options), H1 (final selection) | H1 is the only writer of the final bundle set |
| `conformance_results.json` + module digests | every internal/ and plugins/ change | Pin LAST per change; re-run source-hash tool |

## Readiness notes

- B1 carries the program's only true architectural risk (steering mechanism). Its plan starts with a bounded Eino capability check; outcome decides in-loop vs early-settle (spec §5.2).
- A3 may implement commands whose backend already exists first; `steer/follow_up/queue/tree/clone` stubs return `MethodNotFound` until B1/C1 land — the shim shape is stable before them.
- G3 is face-internal extras; the session-tree navigator lives in C2 (kernel storage via C1).
- G4 is the lowest-value/highest-risk item; it may be descoped to "attach-only" without affecting parity acceptance (pi's inline display is a render nicety, not a capability).
- H1 physically relocates non-protected coding tools (`agent`, `workflow`, `notes`, `network_search`, `http_request`, `web_fetch`, `download`, `sequential_thinking`, goal/plan tools, `job_*`) from `internal/tools` into `plugins/coding/*` — ID stability is the contract; protected IDs never move.

## Decision log (append-only)

- 2026-10-06 VCP-P1: initial package. Owner decisions O1–O8 recorded in spec §2. Steering mechanism left to B1's Eino check. Tool exposure placed in ToolHost (middleware cannot hide tools from the model — it only sees calls at execution).
