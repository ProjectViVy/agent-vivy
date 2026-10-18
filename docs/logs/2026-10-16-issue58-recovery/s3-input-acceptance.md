# Accepted S3 Task1 input preparation

# S3 Task 1 reimplementation handoff

Scope: input admission only. No Task 2 native compaction inference, Finalize CAS, pending commit recovery, installed #66 producer or activation claim is included.

## Candidate and boundaries

- Baseline source: `8ae1db9d5adae06d10ad0b8cb44b519dd30f66d7`; baseline documentation/controller export: `f3be3844`.
- Production fix candidate: `f8d61b315dce27bfdd8a54548c30f872bdf85286`, tree `aae24648366b00f3eedeceeb6d3f36652d46c5b8`.
- App fixture-only candidate: `54f794eb86150709adbc202b082c0957e6bc8381`, tree `3b09b68172654dde16c68a5534d4579c54f7e8f3`.
- Final replay candidate: `96adc4f7b49f25f4f67b1c583597d035ab2cf371`, tree `ef33f3822de62a532d2b5a12421ac22e09d41fdf`.
- Final manifest `source-96adc4f7.sha256`:32 changed internal files, SHA256 `ab4af6bcee375f70e47711dcca4335fb38b86737c4faaac25f7d162919b4a48b`.
- `source-54f794eb.sha256`: 29 changed internal source/test files; manifest SHA256 `66047487dc95b01a6dbf95cd0964d1370e35f33de69e433529aa31fbbe5900bf`.
- Human attribution on all commits: 大湿 <57738020+mastwet@users.noreply.github.com>.
- Source remains on the recovery branch. Root owns publication, durable report copies and index status. No merge, deploy, tenant-data operation, #66 UI/producer implementation or new visible mode/setting was performed.

## Implemented contracts

The existing Service.Run path selects resident preparation solely from trusted singleton/source binding. It calls CaptureSubjectContext once, consuming the producer's domain repair/seal/capture ordering, reads the one bound bundle RecapPort, expands exact suffix originals through one immutable ceiling, validates immutable source/run grants, preserves canonical message identities and includes the current unsealed interaction once. Ordinary Coding retains its existing context/compaction strategy. LIFE code-face admission keeps its trusted FrozenCore, while known children retain a neutral persona and no resident policy.

One host recap_receipt.go codec owns strict canonical receipt validation, projection_policy1, aligned canonical/view-ID pairs and actual-used captured correction subsets. Nonempty automatic summary/reconcile heads without supported certificates are unavailable. Explicit manual edit/restore with empty Sources remain admissible unless covered targets require reconciliation. Clear retains coverage without covered replay. Required reconciliation explicitly refuses before HTTP until Task2 supplies the native adapter. Admission never rewrites or recertifies a legacy head.

Historical automatic projection first checks availability, then known exclusion. Notebook originals and derivatives stay retained and sealable. Complete affected tool groups and inseparable assistant derivatives are omitted from automatic input, with fixed metadata-only System notices; body/title/args/error/paraphrase text is absent. The current authorized live run receives exact complete tool outputs. Native Message.Extra carries existing provenance and disposable admission metadata through checkpoints; there is no SQL acknowledgement cursor or alternate transcript.

Every final provider leg/resume rechecks current reader/source grants, pinned binding and clear epoch, and measures complete message/schema input plus requested output reserve. Missing selected Run pins refuse; known ordinary and child paths remain distinct. Read errors never silently choose legacy compaction. LIFE bypasses legacy reduction/summarization and overflow folding. Capacity shortage does not trim originals.

## Native capability check and approved refinements

Inspected official pinned Eino0.9.20 model.WithMaxTokens, schema.Message.Extra, ADK BeforeModelRewriteState and existing Runner/checkpoint/observer seams. OpenAI ACL v0.1.17 maps common MaxTokens to wire max_tokens; Claude v0.1.25 maps it to MessageNewParams.MaxTokens. Responses remains DEFERRED-INDEFINITE.

Existing resolvingChatModel appended thinking options after caller limits; a local HTTP RED captured Claude max_tokens10240 overriding requested4096. The approved internal LIFE policy owns one4096 requested cap, min with a positive declared model output maximum. The exact native cap is enforced last, and incompatible selected Claude thinking budgets fail before HTTP. No new end-user knob or invented physical capability was added. Unknown context windows/nontext charges remain unavailable. Text uses a labelled conservative UTF-8 byte upper bound with full schemas/framing/reference representations. The final observer checks actual bound tools again.

A trusted admitted provider/model pin is carried separately from physical metadata. Capacity resolves that admitted route instead of mutable CurrentModel labels. Each provider invocation captures one LiveSpec and reuses it for construction, thinking and options. Route mismatch refuses before HTTP. Registered/custom endpoints retain their existing no-implicit-thinking rule. Ordinary no-limit requests retain their original options.

## Verification evidence

Final isolated commands use `/tmp/issue58-localonly-runner.py`, which removes credential-bearing environment names, disables dependency networking and uses a loopback non-forwarding deny proxy; all final command metadata reports0 external requests/forwarding. All commands use `/workspace/scratch/5dda642e718d/toolchain/go/bin` in PATH with `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`, bash login=false and cached dependencies. HTTP bodies contain synthetic fixture data only; they are real provider-adapter wire captures, not real-model quality evidence.

- `54f794eb-focused.jsonl`: `go test -json ./internal/runtime ./internal/app ./internal/provider -run '^TestIssue58' -count=1` exited0;151 passing events,0 failures,0 skips.
- `54f794eb-race.jsonl`: same package/test selection with `-race` exited0;151 passing events,0 failures,0 skips. This validates54 only, before the typed replay refinement.
- `issue58-s3-final-focused.log`: exact96 in isolated detached worktree; `go test -json ./internal/storage/... ./internal/runtime ./internal/app ./internal/provider -run '^TestIssue58' -count=1` exited0 in27.891s;205 pass events,0fail,17skip.14 skips are real PostgreSQL DSN omissions;3 are packages with no selected tests.
- `issue58-s3-final-race.log`: exact same isolated96 and selection with `-race` exited0 in180.34s;205 pass events,0fail,17skip. No race detector findings. Final focused and race each report external rejected0/forwarded0.
- `issue58-s3-1cc-compat.log`: isolated exact1cc; `go test -json ./internal/runtime ./internal/app -run 'Test(Issue58|.*Persona|.*Child|ToolSearchActivatesDeferredTool)' -count=1` exited0 in95.44s with202pass events,0fail/skip. The later bounded legacy presence and empty diagnostic changes receive the full Issue58 gate; this wider result is attributed to1cc only.
- `actual-http-captures.json` reassembles complete request bodies from chunked Go test output; it contains attempts across candidates, each labelled by source log and test. `actual-http-captures-96adc4f7.json` contains96 complete actual HTTP bodies from the two final exact96 commands (48 each), labelled by test/source log; all are fixture data.

Focused actual-wire coverage includes shared equal-text distinct originals and one Recap/current message across close/reopen; complete same-run tool result followed by reopened exact digest, Through1 and idempotent seal; supported/legacy/malformed/unknown projection policy; manual edit/restore/clear; covered archived supersession with unchanged coverage and zero HTTP; complete-domain grant restoration/version/decision denial; grants and clear between model legs; native question checkpoint resume, same-owner/store restart, answer retention and one seal; clear/revoke/missing pin before resume; Notebook current-run access and subsequent automatic exclusion; oversized current text, unknown model/image cost and full schema capacity refusal; exact-read failure; ordinary Coding, LIFE code-face FrozenCore, unnamed first arrival, worker/private/restricted/foreign and narrow-grant isolation.

## Preserved failures and review fixes

Raw RED/build-failure attempts are retained, not overwritten. Important examples:

- shared-red.jsonl: actual baseline provider request omitted shared source originals.
- receipt-red.jsonl / projection-red.jsonl / receipt-subset-red.jsonl: missing codec/projection/subset seams before implementation.
- output-limit-red*.jsonl: fixture build errors followed by actual native cap override RED; output-limit-green.jsonl proves both adapters.
- full-tool-reconcile-red.jsonl: truncated same-run result and missing explicit reconciliation refusal; subsequent exact-wire GREEN retained.
- certificate-refusal-first.jsonl / refusal-truthful-red.jsonl: initial over-specific fixture assertion corrected to the actual runtime refusal contract, then actual truncated 'did not run' refusal RED.
- route-pin-red.jsonl: mismatched admitted provider/model reached HTTP before pin enforcement.
- route-live-spec-first.jsonl: runtime case passed but provider bound(nil) fixture failed; this is not a full-command pass. Provider fixture now supplies a valid nonempty schema and provider-issue58-green.jsonl passes all provider Issue58 cases.
- isolation-red.jsonl: actual LIFE code-face FrozenCore bypass and child LIFE cap inheritance. Unnamed first-arrival content was already correct; its initial keyword assertion was corrected to the established exact prompt literal.
- missing-pin-red.jsonl and missing-pin-leg-resume-red.jsonl: actual missing selected pin bypass, including native second leg and resume; isolation-missing-pin-green.jsonl confirms the fixes.
- f8d61b31-focused.jsonl:146 passing/5 failing events, representing three old app positive fixtures missing newly required Recap/capacity dependencies, plus aggregate parent/package failures. Test-only54f fixtures use the real bound cognitive bundle port and declared capacity; app-fixture-repair-green.jsonl preserves their repaired success/seal/grant checks.

Root's independent fixed3d review reported0 Critical,3 Important and1 fixture Minor. The Important findings were the missing pin guard, LIFE code-face FrozenCore and child policy; f8 addresses each with observed RED→GREEN. Root independently reran six fix groups with21 pass events. That independent scoped result is separate from the initially failing f8 package gate.

## Remaining qualifications

Task2 is not implemented. Required reconciliation and resident explicit compact remain unavailable. Installed LIFE/#66 composition, full CI, real model quality, native Windows, final centralized product acceptance and real PostgreSQL remain unrun/unaccepted here. Earlier root broad execution was interrupted; its partial pass entries are not completion evidence. Only the explicitly approved isolated local-only focused/race and named compatibility selections were run; no outbound workaround was made in this lane.

The replay concern is resolved in1cc and the bounded d550/96 follow-ups. New exact ToolEvidence artifacts persist one explicit typed outcome unit under their existing hash/reference. Replay restores the native nudge failure before tool.finished/model delivery. Logical completion means a durable known outcome, including refused/not_executed; it does not claim tool execution. Native ToolNode crash-cut regression reopens after operation completion/before tool.finished, changes the classifier to allow, then replays the same call with zero invocations, stable receipt and no fabricated terminal/cursor. Real HTTP receives the exact retained refusal.

Legacy blobs remain readable without mutation. Resident replay requires an exact existing host tool.finished for the same call/tool if the blob lacks typed outcome; absent/wrong/malformed proof is unavailable, and body text/current classifier never attest an outcome. Old successful host records with the complete failure unit absent remain admissible. Present null/partial units and null required result/error strings fail. ToolEvidenceOutcome.Validate owns the shared typed enums/effects validation. Explicit empty diagnostics remain valid native behavior (MCP ToolExecutionError may carry empty Text); required diagnostic key, nonnull string, UTF-8 and2048byte limit remain enforced by the new blob decoder.

Preserved additional failures: refusal-replay-red.jsonl showed native refused/not_executed disappearing on same-call replay after classifier change; the first legacy/codec attempts include documented compile/fixture failures before green. `issue58-s3-legacy-presence-red.log` reproduced4 malformed priorhost acceptances, then green. `issue58-s3-empty-diagnostic-red.log` reproduced codec rejection and real Service.Run failure after one invocation/one HTTP; green proves completion, second HTTP, exact typed tool.finished and verified blob after reopen.

`issue58-s3-1cc-race.log` is FAIL(environment):32GB volume full caused SQLite migration failure and runtime compile no-space. Runtime race did not run in that attempt;73 passing/17skip/3failing events do not convert it into a pass. Root reclaimed only stale reproducible Go build cache, preserving modules/source/logs, before final96 validation. `/tmp/issue58-cache-reclamation.json` records that operation.

## Root acceptance and remaining boundary

Root accepts Task1 preparation at96adc4f7 after independent3d review and one TDD fix pass, exact isolated normal/race gates and root execution of the amended recovery proof. This is not combined S3 activation or Issue58 acceptance. Task2 is now in progress using the same C1/C2/native seams. No native fold/reconcile/CAS/pending-receipt retirement behavior is accepted yet.

Optional typed outcome metadata lives in the existing hash-verified evidence blob; no core migration/store was added. Legacy exact host proof is required for resident replay, null/partial proof denies, and a present empty diagnostic preserves existing native behavior. The raw body, arguments, operation receipt and original Journal remain retained.

The complete attempted log snapshot and manifest retain the failed/blocked/interrupted/disk-full attempts. Final normal/race results are for isolated96, not those earlier sources. Wider compatibility is qualified at1cc. PostgreSQL without DSN, native Windows, real model quality and installed #66 are open. Local wire assertions establish request content, not model quality.
