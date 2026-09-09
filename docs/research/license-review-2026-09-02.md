# Reference License Review — claude-code (P3-1) and rig (P3-2)

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 rows P3-1, P3-2; REFERENCE-INDEX open questions RI-OQ-1, RI-OQ-2
> **Method:** direct upstream verification (GitHub API + full raw LICENSE), not second-hand reporting; local `.workspace/` tree status verified on site.

## 0. Prerequisite verification: neither local copy still exists

`REFERENCE-INDEX.md` (2026-08-15 edition) recorded claude-code as `.workspace/claude-code/` and rig as `.workspace/rig/`. An on-site `ls .workspace/` check on 2026-09-02 confirmed that both trees had been cleaned; `.workspace/` now contains only the eight directories agent-wiki-library / caveman / crush / deepseek-harness / eino / headroom / oh-dsh / smoke. Therefore, this review uses the **upstream repositories** as evidence throughout; the index entries were updated accordingly (see §3).

## 1. P3-1 — claude-code upstream LICENSE: proprietary; source reuse prohibited

- **Upstream repository:** `anthropics/claude-code` (the `claude-code-best/claude-code` named by the old local-copy README is a mirror).
- **Full upstream LICENSE.md (obtained through the GitHub raw/contents API on 2026-09-02):**
  > © Anthropic PBC. All rights reserved. Use is subject to Anthropic's [Commercial Terms of Service](https://www.anthropic.com/legal/commercial-terms).
- **GitHub license detection:** `license: None` (the API `repos/anthropics/claude-code` license field is null—GitHub cannot classify it under any OSI license).
- **Ruling:** proprietary software (all-rights-reserved + commercial ToS constraints). **Source copying or derivation in any form is prohibited.** This is consistent with the prior position on 2026-08-06 ("no LICENSE file; treat as all-rights-reserved and use only as an interaction UX reference"), upgraded from an "absence inference" to explicit upstream confirmation.
- **Permitted contact surface (unchanged):** read UX vocabulary and interaction-pattern descriptions at the AGENTS.md/CLAUDE.md level; do not reuse any source files, prompt assets, or schema.
- **Vivy intent remains Defer, with the reason changing from "license unverified" to "license confirmed proprietary."**

## 2. P3-2 — rig upstream LICENSE: standard MIT; "custom license" concern resolved

- **Upstream repository:** `0xPlaygrounds/rig` (Playgrounds Analytics' Rust LLM framework).
- **Full upstream LICENSE (obtained from raw.githubusercontent on 2026-09-02):** standard MIT text, with the copyright line
  > Copyright (c) 2024, Playgrounds Analytics Inc.
The remainder is the verbatim standard MIT license text (use/copy/modify/merge/publish/distribute/sublicense/sell + retention of the copyright notice + AS-IS disclaimer).
- **Root of the historical concern:** the 2026-08-06 index misread the **standard MIT copyright notice** "Copyright (c) 2024, Playgrounds Analytics Inc." as a custom license marker. Full-text review found no BSL / source-available / additional restriction terms.
- **Ruling:** rig is **MIT** and reusable from a licensing perspective. Vivy intent nevertheless remains **Drop**—it is a Rust framework and does not align with V0's Go+Eino choice; removing the licensing obstacle does not change the architectural rationale. If a future SystemV probe needs Rust components, rig is now a "license-safe" candidate reference.
- **RI-OQ-2 closed.**

## 3. REFERENCE-INDEX synchronization

- §3.3 claude-code: add upstream proprietary-license evidence and date; note that the local copy was cleaned.
- §3.15 rig: correct the License conclusion to MIT (misreading corrected); note that the local copy was cleaned.
- §6 RI-OQ-1 / RI-OQ-2: mark RESOLVED (2026-09-02; see this document).

## 4. Net impact on Vivy

Zero code impact. Neither review changes any implementation path; the result is a definitive determination of "which reference projects are off-limits and which are cleared." The existing aggregate-license constraints (REFERENCE-INDEX §4) remain unchanged: source reuse is still prohibited for GPL/AGPL/unlicensed projects.
