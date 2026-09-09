# Diva Capability Inventory (P2-1) — Keep / Adapt / Defer / Drop

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 row P2-1 (`AGENT-VIVY-ASSEMBLY-OPTIONS.md` §5's V0 placeholder is replaced by this document's full inventory); PRD v0 row 23's "deferred to a later artifact" refers to this document.
> **Evidence base:** `morediva/agent-diva` (Rust workspace, 17 crates) inventoried on site on 2026-09-02—README/AGENTS-ARCH.md/LAPUTA.md, each crate directory, `agent-diva-providers/src/providers.yaml` (47 provider presets, grep `- name:`), and `agent-diva-gui/src-tauri/src/commands.rs` (grep `#[tauri::command]` = 176+1). Evidence for each item is annotated inline with a file path; entries that cannot be verified are explicitly marked "unverified".
> **Status semantics:** Each tag is an **inventory recommendation**. Any capability marked "Keep but not delivered" must still go through the capability re-entry process in ASSEMBLY-OPTIONS §6 (proposal → design → architecture decision → implementation → verification) before it may enter Vivy—the tag is not implementation authorization.

## 0. Tag semantics (per ASSEMBLY-OPTIONS §5/§6 and PRD row 23)

| Tag | Meaning |
|---|---|
| Keep | The capability itself belongs to Vivy's product core; Vivy has delivered it or it should re-enter through a proposal. |
| Adapt | Worth having, but it must re-enter in a different form/architecture (not by copying Diva's approach). |
| Defer | Do not work on it now; wait for a real use case or prerequisite capability (most await MEM-1 / the CH track / SystemV). |
| Drop | Explicitly do not carry it forward: license risk, no consumer, or superseded by Vivy's architecture. |

**Vivy current-state reference (2026-09-02):** kernel = session/run/event + eino execution + provider catalog + tools (fs/bash/grep/multiedit/skill) + approval/HITL + compaction + faces/masks + skills marketplace + cron + MCP + token statistics + browser UI (3015) + channel plugins (telegram/discord/feishu/qq/dingtalk/lsp). See the table for the item-by-item comparison.

## 1. Session / conversation / run

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 1.1 | Session CRUD + automatic title | `agent-diva-gui/src-tauri/src/lib.rs:369+` (get/update/generate/delete_session, etc.) | Keep | **Delivered** (internal/session + session RPC + UI; title = session name) |
| 1.2 | Agent loop (context assembly + skill/subagent streams) | `AGENTS-ARCH.MD` CODE MAP; `agent-diva-cli/src/main.rs` | Keep | **Delivered** (internal/runtime + eino ADK) |
| 1.3 | Streaming output + reasoning + tool logs | `agent-diva-cli/src/main.rs` | Keep | **Delivered** (run_events subscription; streaming UI panel) |
| 1.4 | Event bus | `agent-diva-core/src/` (event bus directory) | Keep | **Delivered** (run_events + RPC subscription) |
| 1.5 | Token ledger | `agent-diva-core/src/` token_ledger (internal behavior not read) | Keep | **Delivered** (stats/tokens + token usage in the trajectory projection) |
| 1.6 | Heartbeat / rate limiter / presence | `agent-diva-core/src/` (directory name verified, behavior not read) | Defer | No current consumer; multi-channel presence and similar needs are driven by the CH track |
| 1.7 | audit / audit_parse / audit_sink | `agent-diva-core/src/` (directory name verified) | Adapt | Most of the requirement is covered by D-010 redaction + structured logs; the remainder (audit-stream export) has no consumer |
| 1.8 | supervised / quality / experience modules | `agent-diva-core/src/` (directory names only) | Defer | Adjacent to the MEM-1 family; await a capability proposal |
| 1.9 | Restart recovery / unique terminal state | `agent-diva-agent/` recovery semantics | Keep | **Delivered** (run recovery + TT-2 snapshots; all ASSEMBLY-OPTIONS §8 gates passed) |
| 1.10 | Message edit / rewind / fork (truncation-based regeneration) | GUI command cluster | Keep (not delivered) | **UI-CHAT-ACT in progress**—kernel Journal truncation/branch RPC design slice must come first |

## 2. Provider / model

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 2.1 | 47 provider presets | `agent-diva-providers/src/providers.yaml` (grep `- name:`) | Adapt | Vivy uses a **curated** YAML bundle (D-018); breadth grows only with real consumers, not by copying everything wholesale |
| 2.2 | Gateway-prefix model rewrite | providers/ (same source as the AGENTS.md rules) | Keep | **Delivered** (only true aggregation gateway prefixes; outbound model assertion tests are in place) |
| 2.3 | Custom OpenAI-compatible endpoint | providers/src/ | Keep | **Delivered** (provider catalog + bundle) |
| 2.4 | Model catalog / metadata (including thinking annotations) | providers/ catalog | Keep | **Delivered** (ModelInfo/SupportsImages/SupportsThinking, D9 single source of truth) |
| 2.5 | Transcription (speech-to-text) | providers/src/ file list | Defer | No use case; wait for real demand |
| 2.6 | Retry / request observer | providers/src/ | Keep | **Delivered** (construct a resolving model per call + retry surface; the trajectory exposes retry rows) |
| 2.7 | CLI provider login/switching | `agent-diva-cli/src/main.rs` | Defer | Vivy surface = settings UI + config; no CLI surface |

## 3. Tools

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 3.1 | filesystem / shell / web(grep) tools | `agent-diva-tools/src/` | Keep | **Delivered** (VC-1 tool family: read/write/edit/multiedit/glob/grep/bash) |
| 3.2 | attachment / message / read_tool_result | `agent-diva-tools/src/` | Keep | **Delivered** (attachment VC-1g-2 path; read_tool_result is in the same family) |
| 3.3 | cron tool | `agent-diva-tools/src/` | Keep | **Delivered** (kernel cron + UI + at scheduler UI-CRON-P2) |
| 3.4 | spawn (subagent) | `agent-diva-tools/src/` | Adapt | Child runs (children RPC) are delivered; exposing a subagent as a tool requires a re-entry proposal (approval semantics are complex) |
| 3.5 | ask_user | `agent-diva-tools/src/` | Keep | **Delivered** (HITL-P0 question flow; review/list includes question) |
| 3.6 | planning / update_plan / execution_todo | `agent-diva-tools/src/` | Keep | **Delivered** (RunMode plan + session todo panel + TodoProgressStrip) |
| 3.7 | working checkpoint | `agent-diva-tools/src/` | Defer | The eino CheckPointStore bridge is already in the kernel (D-028); no product-level consumer |
| 3.8 | tool_discovery / skill_view mounting | `agent-diva-tools/src/` | Adapt | skill_view is delivered; TT-1 session-level pin is in progress; broader discovery awaits real scenarios |
| 3.9 | mcp_sdk tools | `agent-diva-tools/src/` | Keep | **Delivered** (MCP management page + tool wiring) |
| 3.10 | memory_* tool family | `agent-diva-tools/src/` | Defer | MEM-1 (DEFERRED) |
| 3.11 | actmem (active-memory write) | `agent-diva-tools/src/` | Defer | MEM-1 |
| 3.12 | wtf tool | `agent-diva-tools/src/` (purpose unverified) | Drop | Purpose unclear + no consumer; if clarified in the future, use a proposal |

## 4. Sandbox / permissions / HITL

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 4.1 | ExecPolicy rule-based approval + approval cache | `agent-diva-sandbox/src/lib.rs:1-18`, `exec_policy.rs` | Keep | **Delivered** (approval center HITL-P0 + three-tier permission presets UI-CHAT-TOOLBAR) |
| 4.2 | Guardian automatic allow | `agent-diva-sandbox/` | Adapt | Vivy's presets (cautious/intelligent/trusted) cover the same responsibility; "automatic-learning allow" and similar items are real SBX needs |
| 4.3 | Windows Restricted Token isolation | `agent-diva-sandbox/src/lib.rs` | Defer | SBX-* (DEFERRED); the approval gate covers the current risk surface |
| 4.4 | Linux Bubblewrap/Landlock/Seccomp | `agent-diva-sandbox/src/lib.rs` | Defer | Same as above (Vivy is currently Windows-first) |
| 4.5 | Persistent approval center (decide/cancel/review + stream) | CLI `approvals`; GUI command cluster | Keep | **Delivered** (approval-center page + run-level approval events) |
| 4.6 | Command-rule CRUD | GUI command cluster | Adapt | Presets cover it; per-rule UI awaits real governance requirements |

## 5. Channels / gateway

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 5.1 | Six channels (Telegram/Discord/QQ/DingTalk/Feishu/Email) | `agent-diva-channels/src/*.rs` | Adapt | Vivy delivers 5 channels as **plugins** (dingtalk/discord/feishu/qq/telegram) + CH-0 contract docs; email and the remainder belong to the CH track (decision: backlog) |
| 5.2 | Retired channels (Slack/WhatsApp/Matrix/IRC/Mattermost/Nextcloud) | README cargo features | Drop | No consumer |
| 5.3 | Gateway = session/routing single source of truth + HTTP control plane | README "How it works" | Keep | **Delivered** (vivy control plane 8787 + message-injection surface) |
| 5.4 | `POST /api/hook/message` external injection | README | Adapt | On the CH track (outbound/replay channels and other CH-0 family items) |
| 5.5 | neuro_link channel | `agent-diva-channels/src/neuro_link.rs` (purpose unverified) | Drop | Same as wtf: purpose unclear |
| 5.6 | Windows service wrapper | `agent-diva-service/` | Defer | Vivy's foreground process + Studio lifecycle cover it; no path for a service form |

## 6. Memory / persona / self-evolution

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 6.1 | BML memory authority (SQLite+FTS5 `.laputa/`) | `agent-diva-laputa/src/lib.rs:1-8` | Defer | MEM-1 (DEFERRED) |
| 6.2 | ACTMEM working memory (ring/capsule) | laputa | Defer | MEM-1 |
| 6.3 | MEMRULES / memory distillation | laputa | Defer | MEM-1 |
| 6.4 | Laputa governance layer (proposal-based writes to authority + rollback + audit) | `AGENTS-ARCH.MD` (only `apply_proposal()` can write to authority) | Adapt | The governance **pattern** has been absorbed by the Studio lifecycle (pack→eval→release→rollback); the memory domain itself awaits MEM-1 |
| 6.5 | Persona Markdown workspace + Frozen Core | laputa | Adapt | Vivy has a persona page + faces track (FACE-TUI-1 F0 paperwork has landed); frozen-snapshot semantics follow the FACE proposal |
| 6.6 | AutoDream proposal lifecycle | `agent-diva-autodream/src/lib.rs` | Defer | MEM-1; UI AutoDream remains a stub (decision) |
| 6.7 | Evolution-controlled skill requests (create/accept/reject) | autodream | Defer | Same as above |

## 7. Skills / files / workspace

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 7.1 | SKILL.md loading (user + repo fallback) | skills loading surface | Keep | **Delivered** (skills directory + enable/disable) |
| 7.2 | Skills marketplace (search/install/upload) | skills.sh panel | Keep | **Delivered** (SKILL-MKT-1 version comparison + in-place upgrade, SKILL-MKT-2 always injection) |
| 7.3 | Skill history/revisions | Skill-revision RPC | Keep | **Delivered** (skills/revisions/list) |
| 7.4 | File index / FileManager / upload | `agent-diva-files/` | Keep | **Delivered** (workspace + file panel + file_versions + attachments) |
| 7.5 | Workspace inspection/switching | GUI workspace command cluster | Keep | **Delivered** (multi-workspace WorkspaceManager) |

## 8. Masks / plans / scheduling

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 8.1 | Masks (persona presets) CRUD/switching | CLI `mask`; GUI command cluster | Adapt | Vivy upgrades masks to real run modes + the Face contract (VC-0c; FACE track); Diva-style text-only presets are not copied |
| 8.2 | Plan approval flow (get/approve/report) | GUI command cluster | Keep | **Delivered** (plan mode + approval chain) |
| 8.3 | Cron CRUD + time zones + channel delivery | CLI/GUI; runs inside gateway | Keep | **Delivered** (cron page + at scheduling); "channel delivery" belongs to the CH track |

## 9. GUI / client form

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 9.1 | Tauri GUI (177 commands) | `#[tauri::command]` grep 176+1 | Drop (form) | Vivy's browser UI (3015 split Vite / 8787 embedded) covers the same needs; individual capabilities are distributed across the corresponding rows above |
| 9.2 | Desktop pet Mate (VRM/TTS/always-on-top window) | lib.rs command cluster (VRM import, MiniMax/SiliconFlow TTS) | Defer | No kernel capability; await a proposal (noted in the UI-CHAT-TOOLBAR row) |
| 9.3 | Speech-synthesis input | Same as above | Defer | Same as above |
| 9.4 | Language switching / splash / GUI preferences | Command cluster | Adapt | Vivy i18n en/zh is delivered; preference persistence grows with UI requirements |
| 9.5 | Log tailing / token panel (in GUI) | Command cluster | Keep | **Delivered** (token statistics page + trajectory panel UI-TRAJ) |
| 9.6 | wipe_local_data | Command cluster | Adapt | Data ownership is a Vivy philosophical anchor (local-first); "one-click reset" follows a settings-page proposal |

## 10. CLI / kernel support surface

| # | Diva capability | Diva evidence | Tag | Vivy status / re-entry destination |
|---|---|---|---|---|
| 10.1 | Full CLI suite (onboard/chat/tui/status/doctor/…) | `agent-diva-cli/src/main.rs` | Defer | Vivy currently has control-plane + UI surfaces; there is no CLI surface. FACE-TUI-1's faces/tui is a TUI **face**, not a Diva-style full-featured CLI |
| 10.2 | neuron (single-turn LLM node primitive) | `agent-diva-neuron/src/lib.rs` | Drop | eino compose/graph already covers composition needs |
| 10.3 | manager (gateway runtime + HTTP control plane) | `agent-diva-manager/` | Keep (different form) | Vivy's cmd/vivy + internal/rpc has the same responsibility |
| 10.4 | migration tool | `agent-diva-migration/` | Drop | There is no Diva user data to migrate |
| 10.5 | e2e real-LLM test harness | `agent-diva-e2e/src/lib.rs` | Adapt | Vivy has full Playwright + Go tests; the "real provider e2e gate" is in use (TEST-1 decision); a heavier harness has no consumer |
| 10.6 | Packaging (NSIS/MSI/deb) | `scripts/package-*.ps1` | Adapt | Vivy distribution = embedded UI exe + split build + Docker; installer follows a release proposal |

## 11. Statistics

68 rows: **Keep 29** (28 delivered + 1 awaiting proposal: 1.10 message edit/rewind/fork) · **Adapt 15** · **Defer 18** · **Drop 6**.

> Note: counts are by row (some rows combine multiple subtools). The "delivered" determination follows 2026-09-02 TODO §10 and `docs/logs/`. **The concentration of Defer items matches the decision:** the MEM-1 family (6.1-6.3, 6.6-6.7, 3.10-3.11, 1.8), SBX (4.3-4.4), and remaining CH-track items (part of 5.1, 5.4). Drop is concentrated in "purpose unclear" (3.12, 5.5), "superseded by architecture" (9.1, 10.2), and "no migration object" (10.4).

## 12. Reiterate re-entry rules (ASSEMBLY-OPTIONS §6)

This inventory does not change any current TODO priority. Any transition from Defer→implementation or Adapt→implementation must first produce a capability proposal (problem statement / workflow / acceptance / safety approval model / state and event semantics / persistence and recovery / UI consequences / explicit tag decision), and **must not** begin from Diva's crate boundaries, Tauri command names, old schema, or backend implementation.
