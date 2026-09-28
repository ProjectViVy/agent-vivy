# ACP-05 Generation and Real Client Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story / Epic:** ACP-05 / E2. **Goal:** Prove a pinned ACP Generation works with a real client, while an omitted ACP source is absent from a second artifact and existing paths still work.

**Architecture:** Seal the final plugin tree with the repository source-hash tool and explicit Recipe pin; Pack both selected and omitted Recipes, Inspect sealed artifacts, inspect Go import graph, and exercise the selected executable as a client-owned stdio child. Attach actual CI and run transcripts. **Tech Stack:** Go 1.26.4, `go run ./sdk`, `just ci`, a real ACP v1 IDE/client, existing Generation tools. **Spec:** [reviewed design](../../specs/2026-09-28-issue1-acp-face-design.md), baseline `3c4ed66`. **State / dependencies:** [index](index.md); requires accepted ACP-04 complete Module, G0 review and G1 scheduling.

## Global Constraints

- `projectvivy/acp`, T2, `std/face@v1`, `core/face-host@v1`, one selected Face, `rpc.client`; no unrequested UI/Host/runtime extension.
- `verify` is static, Pack does not execute conformance tests, and Inspect only reports sealed evidence. A checked-in conformance claim requires the repository's reproduction matrix; unattested is not a failed build.
- Artifact UI assets are still included by current Pack design. Prove omission of Face implementation packages separately from artifact UI content.
- Run `.agents/skills/vivy-plugin/SKILL.md` pressure matrix, `.agents/skills/vivy-kernel-ci/SKILL.md` `just ci`, and the real selected product path; record pass/fail rather than assuming.

## Review Focus

1. Selected Recipe source ref/hash/grant differs by one byte: `verify`/`pack` must reject stale pin instead of silently rebinding it.
2. Omitted Recipe binary must lack `agent-vivy/plugins/acp` and SDK ACP imports; selected artifact must lack `agent-vivy/sdk/tui/face` and `agent-vivy/internal/codeface` (UI assets remain and are reported).
3. Empty/malformed initialize frame and startup configuration error still produce no non-ACP stdout bytes.
4. Real client approval reject/approve, Ask User answer/cancel/expiry and prompt cancellation must settle the exact owning Vivy Run.
5. Existing default gateway, `vivy tui` and `vivy run` behavior must survive the new CLI/overlay path.

---

### Task 1: Source pin and recipe round trip

**Files:** Modify final `plugins/acp/vivy-module.yaml` and provider descriptor hash in `plugins/acp/module.go` (proposed by ACP-03); create (proposed) `recipes/acp.vivy.yml`; modify `sdk/internal/frontend_v1_test.go` to add selected/omitted pack and inspect assertions; add recipe to existing shipped-recipe loop if its external source parameter can be supplied there.

**Interfaces:** Consume final ACP-04 plugin source and `sdk/internal/assembly.HashSourceTree`. Produce Recipe `modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/face-host, projectvivy/acp]` plus required actual providers discovered from Compile; `sources: {projectvivy/acp: {ref: repo:plugins/acp, sha256: <real digest>}}`, `exclusive: {std/face@v1: projectvivy/acp}`, and `grantApprovals: [{module: projectvivy/acp, name: rpc.client}]`. The second artifact uses `recipes/minimal.vivy.yml` without `--source plugins/acp`.

- [ ] Write failing pack tests for selected source/hash/grant/Face and omitted ACP, and one-byte source mutation rejection. Assert Inspect's source pin equals Module descriptor and effective grant, and check selected assembly's Face ID.
- [ ] Run `go test ./sdk/internal -run 'Test(PackACP|PackAndInspectEveryShippedRecipe)' -count=1`; expect missing Recipe/module before implementation.
- [ ] Finalize the descriptor using its canonical ref and declared hash placeholder; compute the digest with `go run ./sdk/internal/cmd/source-hash plugins/acp <declared-sha256>`, then bind the same digest in descriptor, `module.go` and Recipe. Recalculate after **each** ACP source modification; use compiler feedback to add only actually required Module providers to Recipe.
- [ ] Run `go run ./sdk verify plugins/acp`, then `go run ./sdk pack --recipe recipes/acp.vivy.yml --source plugins/acp --output <fresh-temp-acp-dir>`, and `go run ./sdk inspect-artifact <fresh-temp-acp-dir>`. Repeat Pack/Inspect for `recipes/minimal.vivy.yml` into another fresh temp path. Expected: verifiable source/manifest identity, one ACP Face, correct Grant, and successful omitted artifact without ACP.
- [ ] Inspect the exact selected Go build overlay/dependency graph with `go list -deps -modfile <pack-modfile> -mod=readonly -overlay <pack-overlay> ./cmd/vivy` while the overlay and temporary modfile still exist in a focused pack test; `go version -m` on artifacts and generated binder alone are insufficient as proof of omission. Record excluded/imported packages and separately list staged `ui/dist` assets.

### Task 2: Real IDE/client, regression and release record

**Files:** Create (proposed) `docs/logs/<date>-issue1-acp-face/{summary,verification,acceptance}.md`; update [index](index.md) only after evidence. Add targeted process tests under `sdk/internal/frontend_v1_test.go` or `cmd/vivy/acp_test.go` if the real-client exercise exposes a reproducible failure.

**Interfaces:** Consume two Inspectable artifacts and ACP-01/03/04 transcripts. Produce a real-client version/capability transcript, ACP-only stdout capture, real Vivy session/run IDs, stream/review/cancel outcomes and CI result; no claims of checked-in provider conformance absent reproduction.

- [ ] Launch the selected binary from a real stable ACP v1 IDE/client over stdio, capture stdout as NDJSON and stderr separately, run a prompt yielding committed model and tool events, and compare emitted IDs/seq with Vivy's private Journal through the supported Control/replay inspection route.
- [ ] Exercise approve/deny, Ask User answer/cancel/expiry, cancel while an ACP reverse request is outstanding, disconnect, invalid cwd/capability, and a slow/broken reader. Record client/version, observed wire messages, exact Vivy terminal state and any limit preventing a case; do not label an unexecuted case passed.
- [ ] Run the five-command pressure matrix in `.agents/skills/vivy-plugin/SKILL.md`, focused adapter/Host suites and `just ci`; expect all to pass before reporting acceptance. Test the default gateway, `vivy tui` and headless `vivy run` in their normal Generation; for selected ACP, verify `vivy tui` reports unavailable with no TUI implementation linked.
- [ ] Document `verify`, selected/omitted Pack/Inspect, import graph, real client trace, known UI asset packaging and all CI output in the three iteration-log files. Mark ACP-05 Done only with validated A1–A5 evidence and owner acceptance of G1; update Epic status separately in the index. If an integration step fails, fix only that defect and rerun its concrete gate.

**Handoff:** Link the signed/recorded source hash, both artifact manifests, import graph, real client transcript, CI runs and remaining limitations. Source or normative changes invalidate the relevant pin/evidence and downstream acceptance; recompute and rerun before marking Done.
