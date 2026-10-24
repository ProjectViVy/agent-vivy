# H1 — Bundle consolidation (re-scoped 2026-10-07)

**Goal:** the plugin taxonomy lands physically: `plugins/infra/` for local daemons, `plugins/coding/` for programming-specialized capability; `internal/` keeps universal tools; `just ci` green.
**Epic:** H. **Requirements:** RQ-BND. **Predecessors:** all prior stories.
**Spec:** VCP-D1 §5.9 + taxonomy decision (SPEC §2.1, vivy-plugin SKILL "Placement rule").

## Decision recorded (owner, 2026-10-07)

The original H1 tool-migration list was written before the universality rule.
Applying it: nearly every `internal/tools` entry is universal (selected by
every Generation — life mode included), so they stay internal. Moves that
survive the test:

| Asset | From | To | Why |
|---|---|---|---|
| `vivy/local-llm` | `plugins/coding/local-llm` | `plugins/infra/llm` | local daemon infra, not coding-bound |
| `plugins/lsp` | `plugins/lsp` | `plugins/coding/lsp` | programming-specialized tool-world |
| session-tree | `plugins/coding/session-tree` | stays | already correct |

Internal tools **stay** — the whole non-protected set (agent, child_inbox,
reply_parent, commandline, download, http_request, web_fetch, network_search,
echo_info, glob, grep, history_*, job_kill, job_output, list_notes, read_note,
write_note, mcp_list_tools, present_files, reference_context,
sequential_thinking, skill_manage, task_*, workflow, enter_plan_mode,
submit_plan, get_goal, create_goal, report_goal) is universal under the
"every Generation selects it" test. Protected 11 unchanged.

## Tasks

- [ ] `git mv plugins/coding/local-llm plugins/infra/llm`; update `source.ref`
  in vivy-module.yaml + module.go (module ID stays `vivy/local-llm` — IDs are
  source-independent), repoSourceDirs, go.mod require/replace, generate-default
  extern, recipe line unchanged; re-pin module digest in both files.
- [ ] `git mv plugins/lsp plugins/coding/lsp`; same ref/binding updates +
  digest re-pin. LSP also carries DiagnosticObserver/LanguageServerStatusProvider
  binding flags in repoSourceDirs — move them with the entry.
- [ ] Add `vivy/lsp` to `recipes/vivy-code.vivy.yml` — it is currently a
  resolvable module selected by NO recipe (dead inventory); the coding
  generation gains the LSP tool-world. Flagged capability change, owner
  sign-off below.
- [ ] Verify `recipes/vivy-code.vivy.yml` resolves; pack + inspect-artifact
  shows `infra/llm` + `coding/lsp` + `coding/session-tree` inventory.
- [ ] Conformance digests re-pin (plugins set + internal if touched) + pressure matrix.
- [ ] `just ci` full green incl. `TestPackAndInspectEveryShippedRecipe`.
- [ ] Commit `feat(coding): consolidate bundle categories — infra/lsp moves`.

## Boundary

Pure moves. No feature changes. Tool IDs, module IDs, schemas, Journal event
names frozen. Internal universal tools are NOT touched.

## Acceptance

`git log` shows two renames; packed vivy-code Inspect lists all three coding/infra
modules; `just ci` green; tools/list diff vs pre-move is empty.
