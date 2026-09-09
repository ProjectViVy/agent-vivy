# Reference Project Index — AGENT-VIVY V0

> **Status:** Living index of the local `.workspace/` reference projects that inform AGENT-VIVY V0 architecture and capability choices.
> **Updated:** 2026-08-15
> **Owner:** 📋 John (PM) with user (mastwet)
> **Related:** `AGENT-VIVY-DIRECTION.md`, `AGENT-VIVY-ASSEMBLY-OPTIONS.md`, `prd-agent-vivy-v0.md`, `agent-vivy/docs/architecture/VIVY-STUDIO.md`

---

## 1. Purpose

This index records **what each local reference project is, why it is on disk, what AGENT-VIVY may borrow from it, and what is explicitly off-limits**. It is the single entry point before anyone touches another project's code, schema, or assets.

Every entry has the same shape:

- **Path:** where it lives in `.workspace/`.
- **Source:** upstream URL or origin.
- **Language / stack:** primary languages and tooling.
- **License:** verified against the project's `LICENSE*` file (P9-style verification).
- **Why on disk:** reason it was cloned into the local workspace.
- **Vivy intent (`Keep / Adapt / Defer / Drop`):** which Diva-aligned Keep/Adapt/Defer/Drop decision this reference supports.
- **May borrow:** what AGENT-VIVY may legitimately learn or reuse.
- **Must NOT borrow:** what is explicitly excluded.
- **Verification:** the artifact (file path / byte count / commit) used to confirm the claims above.

"Verified" below means we have actually inspected the file (license header + a sample of source), not that we trust memory or a stale note.

## 2. Index

| # | Project | Stack | License | Vivy Intent |
|---|---|---|---|---|
| 1 | crush | Go | FSL-1.1-MIT | Adapt (application-assembly patterns only) |
| 2 | eino | Go | Apache-2.0 | Keep (execution framework) |
| 3 | claude-code | TypeScript | (no LICENSE file detected — see note) | Defer (interactive coding-agent UX reference only) |
| 4 | codex | Rust + TS | Apache-2.0 | Defer (coding-agent UX reference only) |
| 5 | GenericAgent | Python | MIT | Defer (research only) |
| 6 | hermes-agent | Python + TS | MIT | Defer (harness patterns research only) |
| 7 | learn-claude-code | Python + TS | MIT | Defer (educational reference) |
| 8 | MaiMBot | Python | GPL-3.0 | Drop (license incompatibility) |
| 9 | memtle | Rust | MIT | Drop (no Vivy V0 fit; future SystemV probe seed only) |
| 10 | oh-my-pi | Rust + TS | MIT | Defer (coding-agent reference) |
| 11 | openakita | Python + TS | AGPL-3.0 | Drop (license incompatibility) |
| 12 | openfang | Rust | Apache-2.0 + MIT (dual) | Defer (agent-OS reference) |
| 13 | OpenHarness | Python + TS | MIT | Adapt (harness pattern research) |
| 14 | pi | TypeScript | MIT | Defer (coding-agent reference) |
| 15 | rig | Rust | (custom — see entry) | Drop (current API not aligned with V0) |
| 16 | zeroclaw | Rust | Apache-2.0 + MIT (dual) | Defer (personal-assistant reference) |
| 17 | qwenpaw | Python | Apache-2.0 (verified 2026-08-07, not vendored locally) | Defer (V1+ filesystem journal backend design reference only) |
| 18 | deepseek-harness | TypeScript | MIT | Adapt (Studio engine only — discipline and coding-agent surface; not a species dependency) |

The total `find` for source files under `.workspace/` (excluding vendor / target / node_modules / dist):

```
~1.16M lines  (across 16 directories)
~17k Go,  ~64k TS/TSX,  ~16k Rust,  ~3k Python,  remainder misc.
```

These numbers are quick scans, not authoritative totals — they exist only to confirm scale, not to justify any specific reuse decision.

## 3. Per-Project Notes

### 3.1 crush  (`.workspace/crush/`)

- **Source:** vendored copy of `github.com/charmbracelet/crush`.
- **Language / stack:** Go 1.x; `charm.land/bubbletea/v2` UI; `charm.land/catwalk/pkg/catwalk`; `charm.land/fantasy` for provider abstraction; SQLite + `sqlc` for persistence; Go-native pub/sub.
- **License:** FSL-1.1-MIT (`crush/LICENSE.md` — verified, FSL with future-MIT conversion clause).
- **Vivy intent:** **Adapt** — application-assembly patterns only. Specifically: config store, SQLite schema discipline, session/message services, pub/sub broker, permission flow, tool hook pattern.
- **May borrow:** the **shape** of these patterns as described in `crush/AGENTS.md` and observable in `crush/internal/{app,config,session,message,permission,pubsub}/`. Adapt means "Vivy writes its own code following the same pattern", not "import the package".
- **Must NOT borrow:**
  - Wholesale source copy (license risk, scope bloat, and D-021 boundary violation).
  - LSP manager, filetracker, Bubble Tea UI, coding-agent prompts — coding-agent-specific subsystems explicitly excluded by `AGENT-VIVY-ASSEMBLY-OPTIONS.md` §2 B.
  - `charm.land/fantasy` as a runtime dependency for V0 (Eino covers provider + tool abstractions; adding a second framework violates D-021).
- **Verification:**
  - `crush/AGENTS.md` — read.
  - `crush/internal/app/app.go` (2287 lines, 25.4 KB) — read; package layout matches `AGENTS.md`.
  - `crush/internal/agent/agent.go` (2287 lines, 86.3 KB) — read; `RunCompletion` and `dispatchMu` semantics relevant to V0 recovery design.

### 3.2 eino  (`.workspace/eino/`)

- **Source:** vendored copy of `github.com/cloudwego/eino`.
- **Language / stack:** Go 1.18+; modules `adk`, `callbacks`, `compose`, `components`, `flow`, `internal`, `schema`, `ext`.
- **License:** Apache-2.0 (`eino/LICENSE-APACHE` — verified).
- **Vivy intent:** **Keep** as the V0 execution framework.
- **May borrow:** `ChatModel`, `Tool`, `Retriever`, `ChatTemplate`, `ChatModelAgent` / `DeepAgent`, graph/workflow composition (`compose`), callbacks/aspect, `Runner.Query`, interrupt/resume.
- **Must NOT borrow:**
  - Eino's internal types as part of Vivy's public UI contract (D-007).
  - Eino's persistence / config / permission semantics — those are product-runtime concerns and belong to Vivy's own shell.
- **Verification:**
  - `eino/README.md` — read.
  - `eino/go.mod` — module `github.com/cloudwego/eino`.
  - Public surface inspected: `ChatModelAgent`, `Runner`, `DeepAgent`, `ToolsNodeConfig` all confirmed against `eino/README.md`.

### 3.3 claude-code  (local copy cleaned; upstream verified)

- **Source:** upstream `anthropics/claude-code` (the local-copy README's 2026-08-06 claim that `claude-code-best/claude-code` is the mirror; local `.workspace/claude-code/` was cleaned before 2026-09-02).
- **Language / stack:** TypeScript; Bun runtime; VS Code–style architecture.
- **License:** **proprietary (upstream LICENSE.md verified in full on 2026-09-02):** "© Anthropic PBC. All rights reserved. Use is subject to Anthropic's Commercial Terms of Service." GitHub's license detection is also None. See `license-review-2026-09-02.md` §1.
- **Vivy intent:** **Defer** — interactive coding-agent UX reference only. Not a code source.
- **May borrow:** UX patterns and vocabulary read from AGENTS.md / CLAUDE.md level descriptions; not source files.
- **Must NOT borrow:** any source code / prompt assets / schema (explicitly prohibited by the license).
- **Verification:** LICENSE.md full text obtained via the GitHub raw/contents API on 2026-09-02; the `repos/anthropics/claude-code` license field is null; local tree cleanup verified on site.

### 3.4 codex  (`.workspace/codex/`)

- **Source:** `github.com/openai/codex` (per README).
- **Language / stack:** Rust (1,632 files) + TypeScript (511 files); Bazel + Cargo + npm; `codex-rs/` + `codex-cli/` split.
- **License:** Apache-2.0 (`codex/LICENSE` and `codex/docs/license.md` — verified, both reference the Apache-2.0 file).
- **Vivy intent:** **Defer** — coding-agent UX and CLI structure reference only. V0 is not a coding agent.
- **May borrow:** inspiration for command surface ergonomics, terminal UX patterns, and SSE/event-streaming patterns — **not source**.
- **Must NOT borrow:** its full Rust workspace structure (it is a Rust coding agent, not a Go gateway product).
- **Verification:** `codex/LICENSE` header inspected (`Apache License, Version 2.0`).

### 3.5 GenericAgent  (`.workspace/GenericAgent/`)

- **Source:** `GenericAgent/README.md` describes it as "A Minimal, Self-Evolving Autonomous Agent Framework".
- **Language / stack:** Python (71 .py files); modest size (`agent_loop.py` 6.9 KB, `agentmain.py` 18.9 KB).
- **License:** MIT (`GenericAgent/LICENSE` — verified).
- **Vivy intent:** **Defer** — research only. Self-evolving agent loops are not a V0 goal.
- **May borrow:** none in V0.
- **Must NOT borrow:** the self-evolving loop pattern (V0 explicitly excludes AutoDream/Evolution, PRD §4).
- **Verification:** README and LICENSE inspected.

### 3.6 hermes-agent  (`.workspace/hermes-agent/`)

- **Source:** `hermes-agent/README.md` (Nous Research Hermes Agent).
- **Language / stack:** Python (2,012 files) + TypeScript (417 files); large surface.
- **License:** MIT (`hermes-agent/LICENSE` — verified).
- **Vivy intent:** **Defer** — harness patterns research only. Hermes has an opinionated harness shape (skills, plugins, MCP) that overlaps with Vivy's long-term goals but is not a V0 source.
- **May borrow:** none in V0.
- **Must NOT borrow:** skills/plugins/hooks system shape (V0 stays narrow); Hermes-agent's full Python stack (Vivy is Go).
- **Verification:** LICENSE inspected.

### 3.7 learn-claude-code  (`.workspace/learn-claude-code/`)

- **Source:** `shareAI-lab/learn-claude-code` per README.
- **Language / stack:** Python (41) + TypeScript (62); tutorial-structured with `s01_agent_loop` through `s20_comprehensive`.
- **License:** MIT (`learn-claude-code/LICENSE` — verified).
- **Vivy intent:** **Defer** — educational reference for agent harness engineering concepts.
- **May borrow:** conceptual vocabulary from the `s0N` series (when not license-problematic).
- **Must NOT borrow:** its specific module layout.
- **Verification:** LICENSE inspected; README confirms scope is "Harness Engineering for Real Agents".

### 3.8 MaiMBot  (`.workspace/MaiMBot/`)

- **Source:** `SengokuCola/MaiMBot` per README (Chinese-language QQ bot project).
- **Language / stack:** Python (41 files).
- **License:** **GPL-3.0** (`MaiMBot/LICENSE` — verified).
- **Vivy intent:** **Drop** — license incompatibility. GPL-3.0 is viral for derivative works; Vivy must not depend on or copy from this project.
- **May borrow:** none.
- **Must NOT borrow:** anything (license risk).
- **Verification:** LICENSE header inspected (`GNU GENERAL PUBLIC LICENSE Version 3, 29 June 2007`).

### 3.9 memtle  (`.workspace/memtle/`)

- **Source:** `ProjectViVy/memtle` per README.
- **Language / stack:** Rust (119 files); embedded SQLite (turso).
- **License:** MIT (`memtle/LICENSE` — verified).
- **Vivy intent:** **Drop for V0**, retain as a **future SystemV probe seed** (per `AGENT-VIVY-DIRECTION.md` §3).
- **May borrow:** vocabulary and concepts for memory persistence shape in later Vivy stages; **not for V0** (V0 has no memory subsystem beyond simple context replay).
- **Must NOT borrow:** Rust source code (V0 is Go).
- **Verification:** LICENSE inspected.

### 3.10 oh-my-pi  (`.workspace/oh-my-pi/`)

- **Source:** `can1357/oh-my-pi` per README ("A coding agent with the IDE wired in").
- **Language / stack:** Rust (323 files) + TypeScript (3,316 files); very large UI surface.
- **License:** MIT (`oh-my-pi/LICENSE` — verified).
- **Vivy intent:** **Defer** — coding-agent IDE integration reference.
- **May borrow:** UX ideas for tool/IDE integration (post-V0).
- **Must NOT borrow:** its IDE-integration code (V0 has no IDE integration).
- **Verification:** LICENSE inspected.

### 3.11 openakita  (`.workspace/openakita/`)

- **Source:** `OpenAkita` per README ("Open-Source Multi-Agent AI Assistant").
- **Language / stack:** Python (2,229 files) + TypeScript (218 files).
- **License:** **AGPL-3.0** (`openakita/LICENSE` — verified).
- **Vivy intent:** **Drop** — license incompatibility. AGPL-3.0 is viral for network-deployed derivatives; Vivy must not depend on or copy from this project.
- **May borrow:** none.
- **Must NOT borrow:** anything (license risk).
- **Verification:** LICENSE header inspected (`GNU AFFERO GENERAL PUBLIC LICENSE Version 3, 19 November 2007`).

### 3.12 openfang  (`.workspace/openfang/`)

- **Source:** `openfang` per README ("The Agent Operating System").
- **Language / stack:** Rust (260 files); small project footprint.
- **License:** Apache-2.0 + MIT dual (`openfang/LICENSE-APACHE` and `openfang/LICENSE-MIT` — verified).
- **Vivy intent:** **Defer** — "Agent Operating System" framing is interesting for V2/V3; out of V0 scope.
- **May borrow:** inspiration for OS-style agent boundaries (post-V0).
- **Must NOT borrow:** its current Rust code (V0 is Go; V0's product is not an OS).
- **Verification:** both LICENSE files inspected.

### 3.13 OpenHarness  (`.workspace/OpenHarness/`)

- **Source:** `OpenHarness` per README.
- **Language / stack:** Python (379 files) + TypeScript (34 files).
- **License:** MIT (`OpenHarness/LICENSE` — verified).
- **Vivy intent:** **Adapt** — harness pattern research; OpenHarness's `oh` / `ohmo` tooling may inform Vivy's harness direction.
- **May borrow:** vocabulary and harness shape ideas.
- **Must NOT borrow:** Python source (V0 is Go).
- **Verification:** LICENSE inspected.

### 3.14 pi  (`.workspace/pi/`)

- **Source:** `earendil-works/pi-coding-agent` per README.
- **Language / stack:** TypeScript (805 files).
- **License:** MIT (`pi/LICENSE` — verified).
- **Vivy intent:** **Defer** — coding-agent CLI/UX reference.
- **May borrow:** terminal UX patterns (post-V0).
- **Must NOT borrow:** its TypeScript/Node.js stack (V0 uses Go + browser UI).
- **Verification:** LICENSE inspected.

### 3.15 rig  (local copy cleaned; upstream verified)

- **Source:** upstream `0xPlaygrounds/rig` (Playgrounds Analytics' Rust LLM framework; local `.workspace/rig/` was cleaned before 2026-09-02).
- **Language / stack:** Rust.
- **License:** **standard MIT (upstream LICENSE verified in full and corrected on 2026-09-02):** standard MIT terms, with the copyright line "Copyright (c) 2024, Playgrounds Analytics Inc."—on 2026-08-06 this standard MIT copyright notice was misread as a custom license marker; the concern is resolved, with no BSL/source-available/additional restriction terms. See `license-review-2026-09-02.md` §2.
- **Vivy intent:** **Drop** — Rust framework; not aligned with V0's Go+Eino choice (the licensing obstacle being removed does not change the architectural rationale).
- **May borrow:** vocabulary; if a future SystemV probe needs Rust components, rig is now a "license-safe" candidate reference.
- **Verification:** full text of `raw.githubusercontent` `0xPlaygrounds/rig/main/LICENSE` checked on 2026-09-02.

### 3.16 zeroclaw  (`.workspace/zeroclaw/`)

- **Source:** `zeroclaw` per README ("Personal AI Assistant").
- **Language / stack:** Rust (935 files) + TypeScript (94 files) + small Python (15 files).
- **License:** Apache-2.0 + MIT dual (`zeroclaw/LICENSE-APACHE` and `zeroclaw/LICENSE-MIT` — verified).
- **Vivy intent:** **Defer** — personal-assistant framing matches Vivy's gateway philosophy; deferred for post-V0 reuse as a Go→Rust probe seed if/when Vivy explores SystemV ideas.
- **May borrow:** product-philosophy alignment evidence (single-user, local-first, "you own your data") — **not code**.
- **Must NOT borrow:** Rust source (V0 is Go).
- **Verification:** both LICENSE files inspected.

### 3.17 qwenpaw  (verified 2026-08-07, not vendored locally)

- **Source:** `github.com/agentscope-ai/QwenPaw.git` (cloned into `/tmp/QwenPaw` for verification — **not** vendored into `.workspace/`).
- **Language / stack:** Python (large, multi-module); pydantic; SQLite for Scroll history; Git for workspace checkpoint shadow; no Eino dependency.
- **License:** **Apache-2.0** (`/tmp/QwenPaw/LICENSE` header verified, Apache 2.0, January 2004).
- **Vivy intent:** **Defer** — inspiration source for V1+ filesystem journal backend probe; **not** a V0 code source and **not** a vendored dependency.
- **Verified facts about QwenPaw (against `/tmp/QwenPaw`):**
  - Two storage strategies coexist in the same project:
    - **Scroll** (`src/qwenpaw/agents/context/scroll/manager.py`) uses `history.db` SQLite as source of truth. Addendum §2.1 claim **verified**.
    - **Creator Runtime** (`plugins/apps/qwenpaw-creator/backend/services/runtime_files/`) is SQLite-free. Addendum §2.2 claim **verified**.
  - Creator Runtime implementation details **verified** against source:
    - `atomic_store.py` — same-directory temp file + `fsync_directory` (addendum §4.3.a) **verified**.
    - `jsonl_store.py` — append-only JSONL with monotonic `seq`, crash-tail truncation, malformed-rejected-not-skipped (addendum §4.3.b/c) **verified**.
    - `locking.py` — Windows `msvcrt.locking` byte-range locks vs POSIX `fcntl.flock` (addendum §4.3.e) **verified**.
    - `session_store.py` — atomic Pydantic JSON + ordered append-only JSONL (addendum §2.2 directory shape) **verified**.
  - Checkpoint system in QwenPaw (`src/qwenpaw/checkpoints/`) is **shadow-Git-based** (`CheckpointRepository`, `shadow.git`), **not** Eino's `CheckPointStore`. Addendum §6's claims about Eino's CheckPointStore therefore **do not derive from QwenPaw source** — they should be verified against Eino (`diva-go/.workspace/eino/`) directly before being relied upon.
- **May borrow:**
  - The dual-mode lesson (SQLite for indexed history, filesystem for portable project-local state) as design philosophy.
  - The fsjournal implementation patterns (atomic publication, JSONL envelopes, cross-process locking, crash-tail recovery) **as design reference** for a V1+ Vivy filesystem journal backend probe.
  - Specific QwenPaw code: only after a future capability proposal explicitly authorizes it.
- **Must NOT borrow:**
  - QwenPaw's Python codebase. V0 is Go; V1+ probe is independent Go code inspired by the patterns, not translated from Python.
  - QwenPaw's shadow-Git checkpoint design. Vivy's checkpoint story is owned by Vivy and is bridged to Eino, not to Git.
- **Verification:**
  - `/tmp/QwenPaw/LICENSE` — Apache 2.0 verified.
  - `src/qwenpaw/agents/context/scroll/manager.py` lines 108, 332, 556, 564, 1829 — `history.db` as source of truth verified.
  - `plugins/apps/qwenpaw-creator/backend/services/runtime_files/{atomic_store,jsonl_store,locking,session_store}.py` — verified.
  - `src/qwenpaw/checkpoints/repository.py` — shadow-Git checkpoint model verified (NOT Eino).
  - **No `eino` import in `src/qwenpaw/`** (grep confirms), so QwenPaw and Eino are independent projects; addendum §6 Eino claims must be re-verified against `diva-go/.workspace/eino/`.
- **Open follow-up:**
  - RI-OQ-5 **RESOLVED 2026-09-02**: stay external (re-fetch by URL); do not vendor. Rationale and re-evaluation triggers in `qwenpaw-vendor-ruling.md`.

### 3.18 deepseek-harness  (`.workspace/deepseek-harness/upstream/`)

- **Source:** `https://github.com/deepseek-ai/deepseek-harness.git` (shallow clone 2026-08-15, commit `47f9438`, MIT).
- **Language / stack:** TypeScript / pnpm workspaces / vendored Cordis.
- **License:** MIT (`LICENSE` — Copyright 2026 DeepSeek).
- **Why on disk:** evidence and first engine for the independent **Vivy Studio** app (`VIVY-STUDIO.md`). Not a V0 species dependency.
- **Vivy intent:** **Adapt** — take coding-agent tools, profile/bundle assembly, skills/workflows, Windows pwsh, LSP seam. Refuse live in-process plugin OS (`tool-cordis` default off), marketplace, and embedding Node into `vivy.exe`.
- **May borrow:** profile stacking shape; skill discovery ranks; tool catalog *ideas*; sealed factory profile as Studio's first shell.
- **Must NOT borrow:** Cordis as Vivy's kernel; DSH session log as Journal; community plugin discovery; launching Studio from the species.
- **Verification:** `README.md`, `AGENTS.md`, `docs/architecture.md`, `docs/tool-catalog.md`, `LICENSE` read 2026-08-15.

## 4. Aggregate Constraints

Three hard rules apply to every entry above:

1. **No source-copy from projects with restrictive licenses.** GPL-3.0 (MaiMBot), AGPL-3.0 (openakita), and any project with no LICENSE file (claude-code) are off-limits for code reuse. Schema/vocabulary references are unaffected by code license.
2. **No Rust source-copy into V0.** V0 is Go. Rust projects (codex, memtle, oh-my-pi, openfang, rig, zeroclaw) are reference-only until and unless a V3 rebuild is launched.
3. **Crush's FSL-1.1-MIT is permissive but conditional.** Direct source-copy of Crush internals is not in scope for V0 (D-021). The license is recorded so future maintainers can re-evaluate without re-discovering the issue.

## 5. Maintenance

- New entries are added when a new project is cloned into `.workspace/`.
- Existing entries are updated when LICENSE / intent / Vivy alignment changes.
- The "Vivy intent" column is the source of truth for what each project means to AGENT-VIVY. If two reviewers disagree, the PRD's Keep/Adapt/Defer/Drop decisions and `AGENT-VIVY-DIRECTION.md` win.

## 6. Open Questions

| ID | Question | Owner |
|---|---|---|
| RI-OQ-1 | claude-code LICENSE: is there a hidden LICENSE in a subdirectory we missed, or upstream LICENSE in the original repo? Should we ask upstream before treating as "no license"? | **RESOLVED 2026-09-02** — upstream LICENSE.md is proprietary (Anthropic PBC all-rights-reserved + Commercial ToS); see `license-review-2026-09-02.md` §1 |
| RI-OQ-2 | rig LICENSE: is the "Copyright (c) 2024, Playgrounds Analytics Inc." a permissive license with restrictions (BSL-style), or a custom source-available license? Need a real read. | **RESOLVED 2026-09-02** — standard MIT; the earlier reading of the copyright behavior as a custom marker was incorrect; see `license-review-2026-09-02.md` §2 |
| RI-OQ-3 | Are any of the "Defer" projects actually better references than Crush for V0 application-assembly patterns (e.g. OpenHarness)? If yes, swap intent. | user + future architecture session |
| RI-OQ-4 | Should `.workspace/` itself be versioned in morediva, or `.gitignore`d? Currently neither — it's just sitting on disk. | user |
| RI-OQ-5 | Should QwenPaw be vendored into `.workspace/qwenpaw` for future reference, or stay as an external source we re-fetch by URL? | **RESOLVED 2026-09-02** — stay external (external-by-URL), do not vendor; see `qwenpaw-vendor-ruling.md` |
