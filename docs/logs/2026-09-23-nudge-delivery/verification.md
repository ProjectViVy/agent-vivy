# ND-4 Verification Log

Date: 2026-09-23. Host: linux/amd64, Go 1.26.8, Node v22.21 / pnpm 12.5.1, just 1.43.1 (PowerShell recipes via `powershell.exe`). Working tree: `docs/issue58-nudge-design`.

## 1. Acceptance suite (product path, scripted model)

```text
$ go test -timeout 10m ./internal/runtime -run 'TestNudgeAcceptance' -count=1 -v
=== RUN   TestNudgeAcceptanceMissingFileCorrection        --- PASS (0.07s)
=== RUN   TestNudgeAcceptanceCommandRetry                 --- PASS (0.05s)
=== RUN   TestNudgeAcceptanceRemoteToolIsError            --- PASS (0.05s)
=== RUN   TestNudgeAcceptanceBoundedLoopStop              --- PASS (0.06s)
=== RUN   TestNudgeAcceptanceRefusedWriteNoMutation       --- PASS (0.06s)
=== RUN   TestNudgeAcceptanceJournalFailureWhileWaiting   --- PASS (0.04s)
=== RUN   TestNudgeAcceptanceCancelWhileWaiting           --- PASS (0.19s)
=== RUN   TestNudgeAcceptanceParallelOrdering             --- PASS (0.06s)
=== RUN   TestNudgeAcceptanceResumeNoStaleNotice          --- PASS (0.10s)
=== RUN   TestNudgeAcceptancePartialEffectNoReplay        --- PASS (0.06s)
=== RUN   TestNudgeAcceptanceLegacyAndNewPayloads         --- PASS (0.06s)
ok  	agent-vivy/internal/runtime	0.831s
```

First compile surfaced no type errors; first run showed cases B/C/I stalling because `bash`/`write_file` are effectful under `auto` policy — fixed by listing them in `acceptanceOpts.autoApprove` (the same mechanism `vc1_walkthrough_test.go` uses for `write_file`), not by touching production code.

## 2. Plan recipe

```text
$ go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/domain -count=1
ok  	agent-vivy/internal/runtime	23.994s
ok  	agent-vivy/internal/mcphost	0.008s
ok  	agent-vivy/internal/toolhost	0.013s
ok  	agent-vivy/internal/domain	0.002s
```

## 3. Race gate — EXECUTED, NOT PASSED (upstream defect)

```text
$ go test -race -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure' -count=1
WARNING: DATA RACE
Write at 0x00c00043fcd0 by goroutine 1766:
  github.com/cloudwego/eino/compose.(*ToolsNode).Stream.func1()
      .../cloudwego/eino@v0.9.13/compose/tool_node.go:1253 +0x26a
--- FAIL: TestNudgeAcceptanceParallelOrdering (race detected)
```

Re-run with the ND-4 case excluded still fails inside the pinned dependency:

```text
$ go test -race -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure' -skip 'TestNudgeAcceptanceParallelOrdering' -count=1
--- FAIL: TestNudgeContract/EnhancedAdapterBatchOrderAndDurability (race detected, same stack)
```

Attribution: `tool_node.go:1253` is `ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()` — the enhanced converter writes the function-scope `err`, so ≥2 parallel enhanced tool calls race on a shared variable. Every goroutine in both stacks is inside `cloudwego/eino@v0.9.13`; production wraps every tool with `newEnhancedToolAdapter`, so the trigger is parallel enhanced batches, not the nudge code. This is `EINO-TOOLSNODE-ERR-RACE` in `docs/TODO.md` §0.1, first found by the ND-0 contract suite — preexisting, unfixable inside the quarantine (no eino patching/vendoring without a pin exception). Per the plan rule, this gate is **not** reported as passed.

## 4. Full repository gate

```text
$ just ci          # fmt-check ui-ci vet test headless-compile plugin-ci
... ui: 48 files / 393 tests PASS; go vet clean; go test ./... all packages ok;
... sdk/internal 218.2s; sdk/internal/conformance 62.9s; headless-compile ok; plugin-ci ok (exit 0)
```

One interim failure: `TestCheckedInProviderConformanceMatchesExecutedSuites` — the new test file changed the internal source digest (`27710da…` → `7320474…`). Regenerated with the same procedure used by every ND commit:

```text
$ go run ./sdk/internal/cmd/source-hash internal 27710da270963df1743e3cebe17d1371a930696ac8a162617180b1113f82f998
732047466d5be80bdfc4b981976d341ebb2cf68242f33ab740007a52ba872237   # written into conformance_results.json (5 rows)
$ go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
ok  46.084s
```

## 5. Split-runtime smoke (real backend/UI/Journal/tools, scripted provider only)

Commands (isolated; `data/vivy.db`, `data/demo`, `data/workspaces` never touched — `data/` does not exist in this checkout):

```text
$ go build -o ~/smoke/modelstub ~/smoke/modelstub.go && ~/smoke/modelstub   # OpenAI-compatible stub on 127.0.0.1:8399
$ VIVY_USER_HOME=/home/ubuntu/smoke/home DEEPSEEK_API_KEY=dummy-smoke-key \
  VIVY_API_BASE=http://127.0.0.1:8399/v1 VIVY_MODEL=smoke-1 go run ./cmd/vivy   # backend :8787
$ cd ui && pnpm dev                                                            # Vite :3015
```

Scripted provider sequence: `read_file missing.txt` ×3 (identical) → `write_file hello.txt` → `read_file hello.txt` → final text; a `probe` prompt triggers a `write_file` that waits on approval for the cancel exercise.

Browser session at `http://127.0.0.1:3015` (screenshots: welcome/model wizard → chat → approvals panel → completed → cancelled, saved under `/home/ubuntu/screenshots/`):

1. Wizard model step: `openai-completions` / `smoke-1` / `http://127.0.0.1:8399/v1`. Two false starts recorded honestly: the settings selection alone still resolved the deepseek catalog endpoint (a vendor's undeclared base_url keeps the vendor endpoint identity), and adding `DEEPSEEK_API_KEY` froze the documented ENV session — `VIVY_API_BASE` + `VIVY_MODEL` were required to aim it at the stub. Both are product semantics, not test bugs.
2. Session preset Trusted → prompt sent → run `run_3362b503793c9356`: three failed `Read` cards (`filesystem: path does not exist`), `Write hello.txt` awaited approval → approved via the Approvals panel (diff `+smoke-ok` shown) → `Read hello.txt` → assistant text "missing.txt failed three times and triggered the bound; I created hello.txt and read it back: smoke-ok." → `Vivy completed`.
3. Second session (`probe`): `Write probe.txt` awaited approval → Cancel run → `Vivy cancelled`; no file written.

Journal inspection (`/home/ubuntu/smoke/home/vivy.db`, `run_events`, python sqlite3 read-only):

```text
run_3362b503793c9356: 35 events — seq 7/12/17 tool.finished{error, outcome:recoverable, reason:not_found, effects:none};
  seq 18 tool.nudge{tool_call_id:call_r3, tool_name:read_file, reason:not_found, repeat_count:3, template_version:nudge-v1};
  seq 21-23 policy.evaluated(prompt) → tool.approval_required → tool.approval_decided{approved, local_user};
  seq 26/31 tool.finished success; seq 35 run.completed — the only terminal.
run_4770db62e35535ca: 8 events — seq 6 tool.approval_required; seq 7 tool.approval_cancelled{reason:"run cancelled", actor:local_user};
  seq 8 run.cancelled{outcome:cancelled, reason:user_requested} — the only terminal.
All four runs (two pre-fix provider failures included) have exactly one terminal event.
payload LIKE '%dummy-smoke-key%': 0 rows — no credential leakage.
workspace/run_3362b503793c9356/hello.txt exists; no probe.txt anywhere.
```

Per the plan instruction, no claim is made that the model "read" the reminder: the nudge is evidenced only by its scheduling event (`tool.nudge`), the durability-boundary guarantee that it was injected before the next model request, and the unit-level `entry(n)` assertions in the acceptance suite.

## 6. Hygiene

```text
$ git diff --check          # clean
$ gofmt -l internal/runtime # clean
$ go vet ./internal/runtime # clean
```
