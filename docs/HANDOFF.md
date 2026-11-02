# Handoff: continue Vivy development

## Official repository continuation (2026-11-01)

Continue in `C:/Users/Administrator/Desktop/agent-vivy`, with origin
`https://github.com/ProjectViVy/agent-vivy`. The owner promoted the Rust project
after the integration acceptance and archived the Go predecessor. This source
snapshot starts from the destination's existing human-authored license commit;
no experiment branches, tags, Git history, session data or build products are
imported. Product-facing identity is now Vivy. Keep the existing GNU GPL v3
license and third-party notices.

The retained local `vivy-test` repository/worktrees contain historical evidence,
the active trial data and the local Rig checkpoint implementation. Paths and
commit IDs in the dated entries below belong to that historical repository.
The source snapshot was taken from its merged main `4efdd3f`. No old `.git`
directory or reference repository is part of the official upload. The D4 build
still needs Rig checkpoint commit `810d7dfd`; publication of that separate fork
is not included in this repository migration. Its local path remains
`C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable` for verification.

Promotion verification passed 34 root tests and 85 basics tests with terminal
support. Both recipes built from this new checkout using the unchanged local
Rig fork and `CARGO_TARGET_DIR=C:/vtd0`. Native coding acceptance passed at
120x40 and 50x20, including branding, input, tool output/failure and cancellation.
Native persistent acceptance passed prompt editing, trust, persona order, frozen
retries, invalid-file recovery and same-session reopen. All model responses in
these promotion checks used synthetic gateways.

The narrow-screen run exposed unordered whole-world access in default role
composition, introduced by the editable prompt loader. Typed resources/queries
now limit its access; a regression test reproduced the schedule conflicts before
the fix and passed afterward. The existing frozen-selection behavior is retained.
Formatting and whitespace checks passed. The 180-file publication audit found no
tracked build products, sessions, obvious credential tokens or AI attribution
markers. Existing protocol/instruction-file compatibility names remain functional.
Local verification evidence stays ignored under `.workspace/promotion/`.

Provider setup now exposes three Rust-owned desktop presets; see the existing
plan section "Curated provider setup" for defaults and verification. Existing
user configurations remain authoritative.

## Diva six-file core persona (2026-11-02)

Settings no longer presents "persona and prompts" as peers. The `persona` section
is now **Core persona** and shows Laputa's six Markdown files:
`MISSION.MD`, `IDENTITY.MD`, `RELATIONSHIP.MD`, `REDLINE.MD` (required) and
`DREAM.MD`, `DARK.MD` (optional). Sources live under
`.vivy/laputa/personas/main/`; the compiled defaults remain in
`prompts/persona/`. `IDENTITY.MD` absorbs the former `prompts/ASSISTANT.md`,
whose plugin and trust entry are removed. `RECAP.md` and `AUTODREAM.md` remain
internal prompts and are no longer shown in Settings.

- `vivy-laputa` stores schema-2 metadata in `.vivy/laputa/persona.json`, keeps a
  complete immutable six-file snapshot per revision, assembles in fixed order
  behind one `<persona>` block, skips empty optional files, enforces a non-blank
  `MISSION.MD`, 32 KiB per file and 64 KiB assembled, and migrates v1 text into
  `IDENTITY.MD`. External edits are used by the next operation and reported as
  `unpublished` until saved.
- AutoDream candidates are file-scoped to `IDENTITY.MD`, `RELATIONSHIP.MD`,
  `DREAM.MD` or `DARK.MD` and carry the target digest. `MISSION.MD` and
  `REDLINE.MD` are rejected at admission; legacy fileless jobs fail explicitly.
- The desktop protocol replaces `persona_update` with `persona_save_file` and
  returns six-file views plus history; `prompts_read`/`prompt_save` remain for the
  two internal prompts. The panel edits one file at a time, confirms saves to
  protected files, preserves drafts on conflicts and supports revision rollback.

Implemented and plugin-tested so far: the store v2, prompt-chain merge, AutoDream
per-file flow, protocol/contract export and the desktop panel with its unit tests.
The core artifact rebuild, `import-core`, native conversation smoke, desktop
check/test/build and manual WebView verification remain to be run for this change.

## Diva management and native host continuation (2026-11-02)

The active backend and desktop worktrees remain `.workspace/diva-integration`
and `.workspace/agent-diva-next`; do not use the original Vivy checkout. Typed
admission/history and eight management areas now have implementations. Desktop
source is on `feat/diva-core-integration` and consumes an explicit exported core
artifact. No merge/push or release completion is claimed.

Latest evidence: nine native conversation/management cases, five transport cases,
native front smoke, 47 durable Rust tests, 39 Python Rust tests and two DTO contract
tests pass. Desktop production bridge tests pass ten actual cases plus one
subprocess fixture, including real-core outbox write failure, crash/reopen, retained
invalid configuration and second writer rejection; forced host death closes the
Job Object tree. Six Vue runtime tests and production build pass. Browser checks
use explicit synthetic IPC and are not native WebView2 evidence. See the existing
plan's continuation section for commands and remaining gates.

Independent review subsequently fixed credential-policy refresh, empty MCP tool
filters, oversized history frames and uneditable managed candidate admission.
Latest counts are 13 native conversation, 40 Python Rust, 3 Front and 8 Vue cases;
the existing plan records the regression details and environment isolation policy.

Readiness changes now refresh the shared snapshot without requiring a task.
Skill inventories fetch source only when selected. Five artifact-import rejection
tests pass. A short-path clean source/archive build produced a per-user NSIS;
installation, reopening, forced host death, a 0.1.0-to-0.1.1 test upgrade and
default uninstall preserved the disposable session/workspace as expected.
These installation checks used the existing Administrator account and installed
WebView2. Native UI automation still fails with a missing local control pipe.

User-authorized desktop gateway files were read only for isolated live checks.
StepFun's exact model failed with HTTP send errors even without inherited proxies;
the supplied Qwen model returned HTTP 404 `model_not_found`. A subsequent read-only
model-list request returned the exact ID `qwen3.8-omni-flash`. With that ID, the
native core completed streamed chat, a real read-tool call and committed the
expected fixture result. The explicit text, stream and tool probes each passed.
No credential values were copied into records or original gateway files changed.
Native WebView2 interaction/scaling, hidden Recap/wake and clean standard-user
acceptance remain unverified; the complete M1–M6 acceptance is not signed off.

## Diva typed transport continuation (2026-11-02)

Implementation remains active on `feat/diva-desktop`. Rust is now the wire DTO
source; `plugins/vivy-diva-front/contract` contains generated types and examples.
The export example takes an explicit output directory; the contract test rejects
stale checked-in output. Native handshake/boundary checks pass. M2–M6 remain
pending; see the existing plan progress for exact evidence and branch bases.

## Diva transport boundary checks (2026-11-01)

The desktop branch now distinguishes protocol-version and unsupported-operation
errors, bounds reflected IDs, preserves pipe-read error causes, and rejects EOF
without a JSON Lines terminator. Four actual-process boundary tests, the existing
front smoke and all 34 root workspace tests passed. The plan's progress section
records reproduction and remaining work. This does not complete M1: type export
and readiness projection are still pending, as are conversation, desktop hosting,
management and installer milestones. No desktop repository changes were made.

## Diva implementation started (2026-11-01)

The owner subsequently authorized implementation. Work is isolated on
`feat/diva-desktop` in `.workspace/diva-integration`. Initial desktop transport
and recipe build and pass a native pipe smoke; integration remains incomplete.
See the [plan progress](plans/2026-11-01-diva-desktop-integration.md#7-implementation-progress-2026-11-01)
for exact evidence and next steps. Do not treat this as a completed desktop,
conversation or installer. Existing trial data/processes and desktop UI are untouched.

## Diva desktop plan recorded (2026-11-01; not implemented)

The owner requested the [desktop integration plan](plans/2026-11-01-diva-desktop-integration.md)
be written and left awaiting implementation, superseding the preceding request
to implement immediately. Do not treat this documentation turn as implementation.

Selected scope: thin Diva Front over the current Rust persistent core; Tauri-owned
bundled process; close to tray; fresh dedicated data and fixed workspace; complete
local management; Windows x64 installer. Automatic Recap and existing wakes stay
active while hidden. Dream uses existing manual/tool triggers, without a new idle
scheduler. No voice integration or migration of trial data is included.

Backend work belongs here; desktop work belongs in the separate
`.workspace/agent-diva-next` repository. The core still needs the local Rig
checkpoint fork; see the plan for artifact and compatibility boundaries. All six
milestones are pending. Only documentation diff/whitespace review applies now;
no application build, runtime test or installer verification was performed.

## Local main consolidation (2026-11-01)

At the owner's request, local `main` was fast-forwarded from `d834b7b` to
`3136936`, retaining all 14 intervening commits. Every local feature branch is
now an ancestor of main: Plan, provider management, native coding subagents,
Python workbench and initialization were already integrated; D0-D4, setup repair
and editable prompts are now included. All seven registered worktrees had clean
tracked/untracked status at the audit. No remote push or history rewrite occurred.

Continue product work in `C:/Users/Administrator/Desktop/vivy-test` on `main`.
Earlier worktree paths below describe historical execution/evidence locations.
Worktree directories, branch references and ignored data were retained: the
interactive trial still uses the `aedb` worktree's executable and session data.
This consolidation does not migrate or interrupt that trial, combine the coding
and persistent recipes, or merge the separate Rig reference repository.

Verification repeated against main's source: 34 root, 64 basics, 52 durable,
36 managed-workbench and five Laputa Rust tests passed (191 total). The persistent
recipe built against the unchanged local Rig fork, and the native
`scripts/prompt_files_smoke.py` acceptance passed with a synthetic gateway,
including retry-time edits, invalid-file recovery and same-session reopen.
No new real-provider acceptance is claimed. Evidence is ignored under
`.workspace/merge-check/` and `.workspace/d4/consumer/` in the main checkout.

The successful build used `CARGO_TARGET_DIR=C:/vtd0`,
`CARGO_PROFILE_DEV_DEBUG=0`, and `TEMP=TMP=C:/vtd0/tmp`, producing
`C:/vtd0/rig-dev/debug/persistent-agent.exe`. A fresh build in the main checkout's
default `.generated/target` failed inside `aws-lc-sys` with MSVC `LNK1104` opening
an internal linker temporary file; changing to a writable temporary directory
alone did not resolve it. The successful shared-cache build verifies the merged
application, not resolution of that fresh native-dependency build issue.

## Editable prompt follow-up (2026-11-01)

The persistent recipe now selects the assistant role instead of the coding role.
Shipped Markdown defaults are under `prompts/`; [its README](../prompts/README.md)
describes runtime overrides, precedence, trust, update timing and persona import.
Assistant, coding, Recap and Dream prose can be edited without rebuilding when
copied into `RIG_HOME` or the trusted project's `.rig/`. No durable schema change
is required. Laputa's existing revision-checked Markdown import remains authoritative.

Executed checks: 64 basics tests and 52 durable tests passed, formatting and
whitespace checks passed, and the persistent recipe built against the same local
Rig fork. `scripts/prompt_files_smoke.py <binary>` passed against the native Windows
executable and a synthetic gateway: distinct assistant identity, live file edits,
BOM support, untrusted late-created files, persona order, invalid-file refusal and
repair, exact main/Recap/Dream requests across retry-time invalid edits, persona-free
internal roles, and same-session reopen. The new basics regression first failed
against the old implementation. This proves loading/dispatch behavior, not the
conversational quality of revised prose or new real-provider compatibility.

Ignored evidence and a renamed build recipe are under `.workspace/d4/prompts/`.
The existing interactive trial process was left running. Its launcher now selects
`.generated/target/rig-dev/debug/persistent-agent-prompts.exe` on the next launch;
its editable files are `.workspace/d4/manual/home/{ASSISTANT,RECAP,AUTODREAM}.md`.
The original session, settings and credentials are preserved. Close the current
trial normally before reopening it once to adopt the new executable; subsequent
edits to these new prompt files require no restart.

## D4 AutoDream AD-2 (2026-11-01)

Branch `feat/d4-autodream`, based on `334ed9e`, in
`C:/Users/Administrator/.codex/worktrees/aedb/vivy-test`. Use **new D4 sessions**:
the durable state is now `vivy.durable.v3`. Main, production Rig pins and the
local checkpoint fork (`810d7dfd`) remain unchanged. No merge/push is included.

The persistent recipe now compiles `autodream`, which includes D3 waits and D2
context. Other recipes do not acquire it through a runtime flag. Build/run with
the same commands in the historical D3 section below. The coding launcher remains
the ordinary coding recipe. Initialize the Python workbench through `/setup python`
before expecting managed Skill validation; native memory is still optional.

```text
/dream run Review the recent failed calculation
/dream status
/dream detach job:1
/dream cancel
/python rollback <expected-publication-id> <retained-publication-id>
```

Dream is a fixed internal durable agent without `SpawnedBy`, conversational persona
or automatic Recap. It shares the compiled runtime/tools, not foreground task
ownership. Explicit requests coalesce; idle reflection stays off. It can read
committed evidence, author files, submit a candidate, wait, revise or finish
silently. D3 one-shot waits remain available inside Dream. Foreground work takes
priority at operation boundaries; independently admitted checks keep running.

Tools are `dream_request`, `dream_status`, `candidate_submit`, `validation_status`,
`validation_wait`, `validation_detach` and `validation_cancel`. Submission returns
admission, not a passing check. Detach removes the completion wait; cancel stops
the job. Cancelling its owning Dream task also stops it. Cancelling unrelated
foreground work does not. Exit stops the owned validation process tree. A live
host is required; no worker daemon or OS scheduler is added.

Managed Skills contain `SKILL.md`, static `pyproject.toml`, and
`src/import_name/__init__.py` exposing `run`. Validation uses JSON argument/expected
value cases in a fresh process, with optional unchanged old/new regression checks.
The source is frozen and checked before/after execution. The workbench publication
selects sources and environment together. Checks use the full proposed workbench
inventory/bootstrap, including ordinary Skills, to reject binding conflicts.
New model operations and direct tool calls select current; model-planned tools
and retries retain their original selection, even across a later publication.
Workers bind memory settings to the selected manifest and reset variables when
adopting a different publication. Usage events cover the managed `run` binding,
not every arbitrary Python import or side effect.

Source-only changes reuse the prepared environment and existing installation
grant. Changed dependencies wait for the existing setup grant, prepare a separate
environment, revalidate retained Skills, then validate the candidate before
activation. Manual preparation also preserves/rechecks the managed set. Local
validation receipts retain authority for old pinned publications when a later
grant expands scope; project copies alone cannot confer that authority. Explicit
grant revocation is not implemented by this slice.

Evidence and jobs live in durable state plus `.vivy/evolution/`; immutable sources,
staged environments and publication history remain under `.vivy/workbench/`.
The default staging/scratch budget is 256 MiB, with no automatic history deletion.
Dependency environments remain owned by setup. Validation defaults to 60 seconds
(maximum one hour) and 1–16 assertions. The scratch monitor is a process-level
bound, not an OS disk quota or sandbox; one write may exceed it before the next
poll. Published/pinned versions are retained for rollback. Code rollback does not
reverse external effects.

Executed Windows verification: all 15 synthetic native acceptance cases passed,
covering generated repair, old failure/new success, adoption, actual call,
feedback, persona isolation, failed checks, code rollback during model
generation, eight crash/reopen positions (including explicit safe retry), waiter
detachment, foreground progress, cancellation/exit process-tree cleanup, new
dependency setup/staging, and D3/D4 refusals without log changes. Rust checks passed
52 durable, 34 workbench, 63 basics and five Laputa tests, plus 34 root tests.
The D3 feature set still passes its 47 tests and native wake regression matrix.
Python passed 39 tests with 10 explicit native-memory tests skipped; this includes
the offline dependency/space-path regression. Formatting, Python compilation and
Git whitespace checks passed. A separately built ordinary executable passed
adoption and an actual subsequent callable invocation with fault-hook environment
variables set and no diagnostic trace emitted. No real-provider, new terminal UI,
native-memory engine, OS-crash or power-loss acceptance is added by D4. Retained
history and cumulative checkpoint logs remain unbounded; this does not establish
economical multi-day operation.

One fresh independent whole-branch review found four important issues and no
critical issues. One combined fix pass added regressions for full-inventory
validation, retention before manual scope expansion, pinned memory configuration,
and direct-call publication selection. All four regressions reproduced their
failure before the fix and passed afterward. The reviewer inspected source and
evidence and independently checked whitespace; the executor ran the final suites
and complete native matrix after the fixes.

Reproduce with `scripts/rig_fork_probe.py --autodream --rig <rig>` and
`scripts/autodream_smoke.py <vivy.exe> <rig>` (`--only` selects a native case group).
The compatibility group needs the retained D3 executable under
`.workspace/d4/baseline/`. Evidence is ignored under `.workspace/d4/`. One observed
full live fixture used 28 requests, 46.353 seconds and 2,318,500 workspace bytes;
observed first-admission-to-terminal times were 0.890 seconds for initial Skill
publication and 2.564 seconds for repair (polling observations, including adoption);
foreground work during validation took 0.277–0.306 seconds, and while dependency
setup was pending took 0.501 seconds. An incremental diagnostic build took 24.282
seconds; the ordinary build took 23.25 seconds. These are local synthetic
observations, not general performance claims.

### D4 first-run setup correction (2026-11-01)

A manual terminal trial exposed a setup race not covered by the original D4
matrix. While the first Python preparation was running, the disabled-memory
report hid that phase and allowed the startup flow to approve MemPalace. This
replaced the Python grant before the original environment and local validation
receipt finished publishing. Subsequent foreground requests could not select
that environment; the missing retained validation record surfaced as Windows
"path not found", even though the Python executable existed.

Memory setup now reports the shared preparation/publishing phase. Opening or
accepting a setup picker while preparation or receipt persistence is pending
leaves settings/grants unchanged. Missing publication validation errors now name
the local validation problem. Two offline regressions and the old executable
reproduced the failure. The fixed executable passes native first-run preparation,
foreground response and same-session reopen through
`scripts/persistent_setup_smoke.py <persistent-agent.exe>` (test-only terminal
dependencies as below). Rust suites pass 36 managed-workbench, 22 ordinary-workbench
and 52 durable tests. Formatting, Python compilation and whitespace checks pass.

The affected local trial was recovered after matching the manifest digest to its
existing local environment receipt and rechecking the interpreter, package
versions and imports. Its retained validation record was restored with backups;
grants and prior session logs were preserved. A resumed real StepFun foreground
probe received the expected `STARTUP_VERIFIED` response, then was stopped. The
inherited-proxy attempt failed to send; a direct attempt succeeded. The ignored
trial launcher bypasses the proxy for this endpoint only. This is narrow real
response evidence, not complete real-provider tool/evolution acceptance. The
interactive trial was reopened with the repaired binary and original session.

## D3 persistent waits and wakes (2026-11-01)

Branch `feat/d3-durable-wake`, based on D2 `c0bad0d`, in
`C:/Users/Administrator/.codex/worktrees/aedb/vivy-test`. Use **new D3 sessions**.
The D2 executable is preserved at `.workspace/d3/baseline/persistent-agent.exe`;
keep its sessions with that binary. Main and the local Rig checkpoint fork
(`810d7dfd`) are unchanged. No merge or push is included.

```powershell
$env:CARGO_TARGET_DIR = 'C:/vtd0'
$env:CARGO_PROFILE_DEV_DEBUG = '0'
$env:PYTHONUTF8 = '1'
cargo run -p vivy -- build recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
C:/vtd0/debug/vivy.exe run recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
```

The regular coding `start.cmd` remains unchanged. Use the existing gateway
configuration with real model window facts. No new credentials are needed by
the tests. A live terminal/JSON host is required for timely wakes; print mode
persists Waiting and exits. Reopen a live host to process overdue schedules.
This is not an OS service and does not wake a powered-off computer.

Agent controls: `task_control` with wait/seconds/reason; `wake_status` for saved
identity, generations and pending controls; cancel_wake for one outstanding
schedule. A wake resumes the same task's context, and the agent can wait again,
defer or finish silently. Human commands:

```text
/wake list
/wake reschedule wake:task:1 0 60
/wake cancel wake:task:1 1
```

Use the actual ID/generation from list. Commands create no task/Recap material.
The mutation response reports pending state; a subsequent list reads the exact
committed checkpoint. D3 stores schema v2, so removing wake support requires a
new session. Rebuilding does not migrate existing sessions.

Executed Windows verification: 34 root tests; 47 offline checkpoint/D1/D2/D3 tests
(35 also passed with the D2 feature set), including
repeated waits, nine fire/delivery reopen positions, saved-control reopen,
tool-boundary yielding, target-only cancellation, stale generations, UTC changes,
failed checkpoint append and immutable/deduplicated Recap sources. Native Windows
smoke has passed repeated waits -> real shell -> silent finish, eight forced
process exits around control/schedule/fire/input, print-mode waiting exit, and
D1/D2 version refusals in both directions without log changes. The management
list/reschedule/cancel sequence created no extra task or model request. D2's
native context/Recap/no-Python and optional real native memory matrix passed,
including a killed memory write reconciled with get rather than replay. D1's
nine crash points plus shell/Python cancellation and ordinary coding smoke
also passed. Formatting, Python compilation and Git whitespace checks passed.

Reproduce with `scripts/rig_fork_probe.py --wake --rig <rig>`,
`scripts/durable_wake_smoke.py <vivy.exe> <rig>`, and the existing D1/D2 smoke
scripts (which now explicitly derive their original feature sets).
Evidence is ignored under `.workspace/d3/`. The two-wait/one-tool fixture used
four model requests, 14-15 ms wake lateness and 2,369,465 bytes of cumulative
logs; diagnostic rebuild took 10.504 s. Reopen took 0.631-1.985 s with a saved
control/fire, or 3.933 s when an uncertain control needed status inspection and
resolution. These local synthetic observations do not establish long-running
storage or general latency. The ordinary production executable also rebuilt;
diagnostic fault hooks are absent without checkpoint-probe. No new terminal
ConPTY, real-provider, OS-crash or power-loss acceptance is claimed. AD-2 and
external channels remain outside this phase.

A fresh independent read-only review inspected the complete D3 change and Rig
inbox/clock behavior, finding no substantiated issue requiring correction. It
inspected the run evidence and independently checked Git whitespace; it did not
rerun the runtime suites. Multiple simultaneous overdue targets were assessed
from the implementation, while the executed boundary test uses one overdue
target during another task's tool. The production build's separate live JSON
smoke passed wait/wake/silent completion with exit zero and no diagnostic trace.

## D2 persona, Recap and explicit memory (2026-11-01)

Current branch: `feat/d2-assistant-context` in the same isolated worktree, based on
`2eb70d6`. Rig remains `810d7dfd` in the path below. Main and the Rig fork are
unchanged; no merge/push is part of this delivery. The previous D0/D1 section is
historical. A D1 executable is preserved in `.workspace/d2/baseline/`.

```powershell
$env:CARGO_TARGET_DIR = 'C:/vtd0'
$env:CARGO_PROFILE_DEV_DEBUG = '0'
$env:PYTHONUTF8 = '1'
cargo run -p vivy -- build recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
C:/vtd0/debug/vivy.exe run recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
```

Configure a real gateway through the existing settings/provider flow, including
its actual `context_window` and output limit. No credentials are supplied here.
Use a new session; do not resume D1 or coding sessions with the D2 binary.
The default `start.cmd` still launches the coding recipe. No product face was
changed by D2.

Commands: `/persona show [NAME]`,
`/persona update <expected-revision> <NAME> <text-file>`,
`/persona rollback <expected-revision> <target-revision>`, `/recap status`, and
`/recap refresh`. Management commands are not tasks or summary sources. Persona
updates affect new operations, not retries. New input preempts background Recap;
there may be a gap until recent closed tasks reach the committed summary. No raw
chat tail or automatic retrieval fills that gap. On summary failure, inspect
status, correct the cause and refresh. Print/JSON foreground completion does not
wait for automatic maintenance; explicit JSON refresh and recovery of saved
maintenance require keeping that process alive.

`memory-tools` is selected in the persistent recipe, with memory disabled by
runtime default. Existing `/python prepare` and `[python] memory=true` configure
the same native backend as before. Removing the Python component and rebuilding
removes the workbench and typed tools entirely; Recap still works. Do not use
private palaces for acceptance.

Executed Windows checks (synthetic loopback unless noted):

- Root: 34 tests; durable checkpoint/D1/D2 consumer: 35 tests; Laputa: 5 tests;
  Rust workbench with typed tools: 22 tests; prepared Python/native suite: 45
  tests. Offline model tests require no keys. Reproduce the durable consumer with
  `python scripts/rig_fork_probe.py --assistant-context --rig <rig-checkout>`.
  The prepared interpreter on this machine can write its native asset root but
  not the default Windows temporary directory. Python tests passed with `TEMP`
  and `TMP` set to that isolated root's `tmp`; the first run failed at temporary
  file creation. The system Python has an incompatible MemPalace installation;
  use the prepared environment and `VIVY_MEMORY_TEST_ROOT`, not system packages.
- `scripts/assistant_context_smoke.py <vivy.exe> <rig>` builds the ordinary and
  no-Python diagnostic variants, captures actual requests, checks tool continuity,
  raw-history exclusion, persona updates, transient summary retry, preemption,
  four summary crash boundaries and unchanged D1 logs on incompatible reopen.
  `--memory-assets <isolated-prepared-workbench-root>` adds real native
  remember/get/search and a process-killed write reconciled with get, not replay.
- The recorded joint run used 25 model requests (294,205 serialized request
  bytes). New foreground completed in 0.215 s while a summary response was blocked.
  Reopen took 2.005–2.161 s when a model retry was needed and 0.081–0.240 s with a
  saved result/publication. Warm diagnostic builds took 8.639 s and 5.420 s;
  these are observed local runs, not general performance claims.
- The three-task fixture's log was 1,557,448 bytes. Cumulative snapshots remain
  a significant retention cost; disk use is not bounded by Recap context size.
- `scripts/smoke.py` retains coding gateway, tools, Python state/reset, sessions,
  JSON, error, credential filtering and model-fact behavior. The D1 crash matrix
  explicitly derives a D1-only variant after this recipe change.

Evidence is ignored under `.workspace/d2/`; the implementation plan records the
final review. No new real-provider compatibility is claimed: the prior owner
ruling deferred provider verification, and no complete authorized replacement
credential/model pair was available. Existing Rig Windows baseline failures
remain as recorded under D0/D1. D3 sleep/wake and AD-2 evolution are still future
work. Log retention deserves measurement before a long-running deployment.

## D0/D1 local runtime (2026-10-31)

The grouped-checkpoint D0 consumer and minimal D1 runtime now run through the
actual Windows builder, launcher and executable. Earlier blocked status below is
historical for the pinned upstream. The new path explicitly requires the editable
Rig fork; no production pins or checked-in recipe locks were replaced.

Implementation is in the existing Vivy feature worktree
`C:/Users/Administrator/.codex/worktrees/aedb/vivy-test`, branch
`feat/d0-rig-probes`; Rig is
`C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable`, branch
`feat/durable-checkpoints`, checkpoint implementation commit `810d7dfd`.
The original Vivy main checkout is unchanged. No push
or PR is part of this delivery. Run from the feature worktree:

```powershell
$env:CARGO_TARGET_DIR = 'C:/vtd0'
$env:CARGO_PROFILE_DEV_DEBUG = '0'
cargo build --offline -p vivy
C:/vtd0/debug/vivy.exe build recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
C:/vtd0/debug/vivy.exe run recipes/persistent-agent.yaml --rig-path C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable -- -p "hello"
```

The last command uses the normal gateway settings and environment-variable
credential reference; it needs a configured endpoint. No credential is supplied
by the recipe. Omit `-p "hello"` for the coding face, or append
`--resume <session-id> -p` to resume unfinished work without new input. Use the
same workspace on resume. A finished session with no prompt exits 2 normally.
Development builds keep their own generated package, Cargo.lock and `rig-dev`
target directory; metadata rejects mixed Rig sources. Rebuild after source edits.
An old coding session is not converted. Durable fork/rewind/copy/import is refused.

Reproduce offline Rust and synthetic loopback acceptance:

```powershell
$env:PYTHONUTF8 = '1'
python scripts/rig_fork_probe.py --runtime --rig C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
python scripts/checkpoint_smoke.py C:/vtd0/debug/vivy.exe C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
python scripts/durable_runtime_smoke.py C:/vtd0/debug/vivy.exe C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
```

Executed evidence is summarized in the [plan](plans/2026-10-31-d0-rig-probes.md)
and [storage research](research/2026-10-27-session-storage.md#grouped-checkpoints-and-d1-2026-10-31).
The process-kill matrix covers nine intent/effect/result/completion boundaries.
Intent-only unsafe tools stay unknown, saved results are reused, competing writers
are refused, and actual Shell/Python workers stop on human cancellation with no
restart replay. Missing-plugin, invalid-state and complete-corruption entry tests
preserve the original logs/files. Ordinary coding/provider/subagent/ownership
smokes pass. Rig retains four reproduced Windows baseline stage-test failures;
the full ECS suite is not green. Real-provider, OS-crash and power-loss behavior
are unverified and are not acceptance claims.

Next is D2: persona and asynchronous Recap-only context. D1's current conversation
context is only an execution fixture, not that completed policy. It includes no
delegation, scheduler, AutoDream, owner distinction or automatic memory recall.

## Requirements audit: owner rulings accepted (2026-10-31)

Read the [accepted rulings](DESIGN.md#requirements-audit-and-owner-rulings-2026-10-31).
The owner accepts ordinary process-crash recovery as the first guarantee and
asynchronous Recap publication using the latest committed version. Pending sources
are retained, not injected, and summary lag does not block new turns. Stronger
sync is a later option, not a first-phase gate.

The owner corrects the delegation comparison: the persistent agent is new, with
its own lifecycle on the shared Vivy/Rig core. Coding subagents are a separate
facility. First-phase scope excludes persistent delegation and coding-session
creation/management; do not add a child-join milestone to catch up with coding.
The existing coding face can validate shared host/tool infrastructure without
making coding sessions part of the deliverable. Later preference: the persistent
agent manages coding sessions rather than directly managing their subagents.
That preference is not an API/implementation lock. AD-2's separate internal Dream
context and validation jobs do not become general subagent delegation.

Reuse existing Rig/host autonomy/resource controls where applicable; other
mechanisms and numeric defaults are deferred. Owner/outsider differentiation and
role-specific context policy are also deferred, including the earlier audit's
proposed isolation. Preserve normal event identities and explicit human stop.

DESIGN, the D0 plan and storage proposals reflect this contract. At the audit,
checkpoint/result ordering and required-state validation were missing. The local
implementation and repeated process-crash acceptance above now address those
gaps; relaxing disk sync alone was never evidence of a pass.

## Assistant context clarification (2026-10-31)

The owner's question distinguishes mysoul's Recap-only history from its optional
memory-reading tools. The assistant policy is persona/instructions + committed
Recap + current input/task, with no automatically included old raw-chat tail or
retrieved evidence. The earlier recent-tail suggestion is superseded. An active
task still retains its execution context and unresolved tool-call/result pairs.
Memory reads remain separate optional tool calls; recap must work without them.

[DESIGN](DESIGN.md#assistant-context-clarification-2026-10-31) records the corrected
boundary and D2 acceptance. Rust still owns the effective context and reuses Rig's
summary/storage primitives; ordinary coding compaction is unchanged. This is a
documentation correction, not a completed Recap integration. Continue D0/D1
storage work, then implement and inspect the assistant's actual model inputs in D2.

## Rig fork: saved serialization failure (2026-10-31)

The owner authorizes modifying `mastwet/rig` for the missing D0 seams. The editable
checkout is `C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable`, branch
`feat/durable-checkpoints`, based on audited Rig `fc556f982d501eea9d7167398a19386e4884e776`.
This permission supersedes earlier no-upstream-patch instructions for these fixes;
the existing reference checkout remains unchanged. Nothing has been pushed.
Local Rig commit: `1ed6ee437e772ea120f2efadf9aa6001e5db67b5`.

The first patch adds `Writer::fail` and routes saved component/resource serialization
failures into the existing failure latch. It preserves the first cause, blocks all
pending log writes and later saves, and keeps earlier bytes readable. Local notices
retain the saved key/log/cause; tools refused because storage failed get a generic
result so serializer diagnostics cannot disclose saved data to the model. A missing
doc comment on Rig's non-Unix `kill_group` was also fixed to compile the path dependency
on Windows; that function's behavior is unchanged.

Vivy validates this with an isolated manifest, without changing its production pins:

```powershell
$env:CARGO_TARGET_DIR = 'C:/vtd0'
$env:CARGO_PROFILE_DEV_DEBUG = '0'
python scripts/rig_fork_probe.py --rig C:/Users/Administrator/Desktop/vivy-test/.workspace/rig-durable
```

Omitting `--rig` runs the same acceptance tests on pinned upstream and reproduces
the failure: a tool starts despite lost saved state. The fork refuses that dispatch.
Final fork and normal pinned probe suites both pass 12 tests; Python checks pass 5.
Rig memory passes 80 tests. ECS passes 129, with 4 failures reproduced unchanged on
the baseline. Scoped Clippy and the two crates' WASM compilation checks pass.
Evidence is under `.workspace/d0/fork-probe/`; `rig-fork-probe` selects diagnostic
tests only, adding no runtime capability. The ordinary `d0-probe` feature keeps its
original upstream characterizations. The normal D0 report is still about the pinned
upstream, so it must not claim this local patch has already shipped there.

This patch does not supply complete checkpoint receipts, final-result gating,
required saved-key validation or D1. The current target is process-crash recovery;
stronger synced receipts are later work. Logging failure is not automatic
cancellation of a normal turn.
Next is the committed checkpoint/receipt seam, followed by the remaining D0 checks.
Verification and the baseline Windows test failures are recorded in the
[session-storage evidence](research/2026-10-27-session-storage.md#rig-fork-saved-serialization-failure-2026-10-31).

## D0 follow-up: session ownership enforced (2026-10-31)

Following the owner's request to continue, the first local D0 gap is addressed.
Generated executables acquire `vivy-process::session::SessionGuard` before Rig
or recipe plugins are constructed. Direct runs own Rig's session lease and a
small execution lock. A launcher keeps the session lease and hands execution to
its checked child; the child validates its actual parent/run and held leases.
An execution lock also protects the interval between launcher loss and child
cleanup. It lives at `RIG_HOME/sessions.vivy-locks/<session-id>`, outside the
deletable session directory; these small lock files are deliberately retained.
This is trusted-process coordination, not a sandbox or durable commit.

CLI fork holds both source and target, excludes process lock files, and refuses
a running source before creating a copy. Ephemeral cleanup reacquires ownership,
renames the directory under the external lock, then removes only the retired
directory. It retains sessions owned by another process and cannot delete a new
session created at the original path. Launcher children
receive fresh protocol state; standalone nested execution that inherits only
RIG_LAUNCHER remains supported. Host ownership is mandatory generated wiring;
YAML still selects all optional plugin capabilities at compile time.

Reproduce with `python scripts/session_ownership.py C:/vtd0/debug/vivy.exe`
after the build commands in the [D0 evidence record](research/2026-10-27-session-storage.md#d0-execution-evidence-and-bounded-proposals-2026-10-31).
The fixture covers competing launcher/direct processes, live-source fork, invalid
handoff, normal exit, killed executable/launcher, nested entry and ephemeral cleanup.
An empty `RIG_DESCRIBE` still requires ownership, matching Rig's describe predicate.
Independent review found that bypass and a cleanup race; both have failing
regressions followed by verified fixes. On Windows, holding an internal lock file
prevented directory renaming, which is why the execution lock is external.

The follow-up root suite passes 29 tests, including a deterministic retirement/
new-owner interleaving; basics passes 82, D0 Rust probes 12 and Python checks 5.
Builds and the assembled ownership fixture pass on Windows. The final D0 rerun
returns the expected exit 2 with four gated starts and zero disk receipts.
Earlier coding, provider, optional-recipe and terminal cancellation smokes also
passed during this follow-up. This does not add non-Windows or real-provider evidence.

Scope is generated entry, CLI fork and ephemeral cleanup. Upstream maintenance
writers (`edit_stopped`, `rewrite_stopped`, `DeleteSession`) still check only Rig's
session lease; they do not consult the external execution lock during the interval
after launcher loss and before child exit. Arbitrary Store callers and old binaries
are also outside this guard. Rebuild generated executables to adopt it.
The D0 report now lists host ownership as supported; its remaining storage/result/
required-state blockers still return exit 2. Keep D1 blocked. No Rig source patch,
dependency upgrade, private Store or real-provider retry accompanies this change.

## D0 implementation and blocked persistence gate (2026-10-31)

The [D0 plan](plans/2026-10-31-d0-rig-probes.md) is implemented. Every active Rig
dependency/recipe is pinned to `fc556f982d501eea9d7167398a19386e4884e776`;
Rust 1.97.1 and Bevy 0.20.0 remain unchanged. TUI adapters use LookLine and current
highlighting APIs. Context/Plan preparation has explicit ordering even when the
recipe excludes subagents. Ordinary restored workflows stop, without silently
launching a new model request; unsupported invocation flags now exit 2.

The new `plugins/vivy-durable` crate exposes only feature `d0-probe`, for synthetic
diagnostics. `scripts/durable_probe.py` derives a temporary recipe and runs Rust
characterizations plus actual launcher/species checks. It returns **2 for known
persistence blockers**, with evidence in ignored `.workspace/d0/report.json`.
It does not implement a DurablePlugin, sync receipts or the D1 state machine.
The default recipe's dependency graph contains no vivy-durable.

Verification on Windows, MSVC, isolated `C:/vtd0` target (debug symbols disabled):

| Check | Observed result |
| --- | --- |
| Root workspace / basics all features / gateway / Python Rust tests | 22 / 82 / 33 / 20 passed |
| D0 Rust characterizations / Python checker/environment tests | 12 / 5 passed |
| Baseline coding recipe | Built; final incremental rebuild 5.89 s, not a cold-build comparison |
| Coding / provider / subagent loopback smoke | Passed; two provider wire families, unchanged model IDs, tools, failures and sessions |
| Native terminal smoke | Passed at 50x20: Plan UI, markdown, tool failure and cancellation |
| Python workbench lifecycle | 20 cycles, zero orphans; cold median 1.186 s, warm 0.082 s, maximum close 0.322 s in this fixture |
| D0 coding and entry paths | Four gated starts; unsupported Rig format refused unchanged; launcher lease enforced; required-state and direct-entry gaps reproduced |
| Compile selection | Headless delegation and no-subagent recipe smoke passed; disabled probe feature checked |

Independent review identified inherited Rig environment contamination and an
omitted routed-model identity in the diagnostic fingerprint. Both received
failing regressions, fixes and focused suite/assembled-path reruns. Same-identity
opaque endpoint changes remain explicitly outside the probe's guarantee.

The lifecycle fixture is not a rerun of all native MemPalace integration tests.
These are synthetic/offline model tests, not real-provider or power-loss evidence.
The TUI check used an ignored task-local venv for pyte/pywinpty. Reference checkouts
and user environments were not changed. Full reproduction commands, patch-sized
proposals and remaining unverified cases are in the
[D0 evidence record](research/2026-10-27-session-storage.md#d0-execution-evidence-and-bounded-proposals-2026-10-31).

**Next:** discuss/implement the remaining bounded upstream seams in that record,
then repeat D0 before D1. The owner selected proposals on reproduced
gaps for this delivery; no private Store, upstream patch, issue/PR publication,
push or deferred real-provider retry was performed. Older entries below describe
their own dated checkpoints; their "next plan" and "pin unchanged" statements do
not supersede this implementation record.

## Consolidated architecture baseline and review (2026-10-31)

Read the [consolidated baseline](DESIGN.md#consolidated-architecture-baseline-2026-10-31),
its [execution contracts](DESIGN.md#preliminary-detailed-architecture-baseline-2026-10-31)
and [AD-2 contracts](DESIGN.md#ad-2-integration-contracts) first. The owner requests
final synthesis, review and initial lock of the recent decisions. Independent
builder review and a focused publication-race recheck found no important planning
blocker; the preliminary detailed architecture is now locked. This checkpoint
supplies design, not runtime delivery. Older entries below are chronological
evidence, superseded where they conflict with this baseline.

One Rust/Rig core, Plan B, native Laputa persona, Rust-owned Recap, existing
Python-native MemPalace, agent-led waiting and AutoDream AD-2 remain the selected
directions. The synthesis fixes initial ownership, per-agent context, single-tool
handoff, uncertain-effect recovery, internal job delivery and tested-version
publication. The private framed Store is still a D0-dependent fallback.
Dream has a distinct durable agent context on the same core; no general durable
subagent joins, official inbox or second runtime are needed to admit its work.

AD-2 requires staged preparation, an environment-plus-code publication identity,
shared publication locking, retained usable revisions and recovery by update ID.
Preserve startup validation before interpreter execution. Generated managed Skills
within the enabled evolution/dependency scope use automatic adoption; the old
blanket added-Skill prompt must not become per-candidate approval. Unadmitted
dependency/source changes still use existing setup, without blocking all Dream work.
Actual loaded Skill revision/results return to collection.

The assistant context clarification above supersedes the synthesis's recent-tail
suggestion: use Recap-only cross-turn history, separate per agent, and no automatic
retrieval. Reuse Rig primitives and retain ordinary coding compaction.
PEN/NeuroLink positioning and selected official QQ/NapCat/Minecraft roles remain
locked, while package/wire details and the game backend remain integration work.
Minecraft can use proven execution/job seams without waiting for all of D4.
Product navigation/Diva form and the later inbox schedule remain parked.

Next produce an implementation plan beginning with D0/D1 and prove the Rig seams
at a deliberately chosen pin before adding D2 context, D3 wake and D4 evolution.
Offline failure injection may proceed without treating the deferred real-provider
joint acceptance as passed. Do not automatically retry that deferred provider run.
No product code, dependency pin or reference checkout changes accompany this design.

Verification: reviewed the documentation diff, passed `git diff --check`, and
checked seven added/changed local links plus code-fence balance across the three
changed documents. Independent builder review and its final focused recheck
reported "nothing important left." No build, crash experiment, gameplay or
real-provider run was performed for this documentation deliverable.

## AutoDream AD-2 selected (2026-10-31)

The owner selects [AD-2: file snapshots and background validation](DESIGN.md#ad-2-file-snapshots-and-background-validation-owner-2026-10-31).
Dream remains an independent context/task on the same Vivy core and chooses its
own work. The host owns background validation jobs, cancellation and result
delivery; admission is not successful validation. Automatically publish the
tested code revision through the existing owner, retain a previous usable
version and return later use/results as evolution evidence.

Reuse the current workbench, process ownership and durable task path, adapting
small Prime mechanisms. Separate code revisions from dependency environments;
publication detection must include the Skill revision, and worker replacement
resets transient Python state. Start with one validation job. The official inbox,
general hooks, a Git artifact repository and another agent runtime are not needed.
The existing durable-core Plan B and single-tool handoff/human stop decisions
remain unchanged. The later synthesis above settles initial publication/recovery
contracts; exact APIs/files remain builder choices. No implementation or runtime
acceptance is delivered here.

## Executable Skill evolution required; Prime reuse audited (2026-10-31)

The owner includes executable Skill generation, validation, automatic adoption
and later-use feedback in AutoDream's acceptance scope. This supersedes the
persona/text-only first-slice proposal below. Read the
[Prime executable Skill audit](research/2026-10-25-hybrid-agent-workbench-decisions.md#25-prime-executable-skill-reuse-for-autodream-2026-10-31).

Remote main `c6d4da9421e6087e3fc1a763e947cf37153cd76f` supplies authoring guidance,
Python package conventions, callable binding/bootstrap, command quality checks
and refinement entry conflict/history/rollback mechanisms. It does not establish
a complete executable-package test/publish/rollback/usage-feedback workflow.
Reuse current Vivy discovery/preparation/loading; propose small adaptations and
tested-revision publication instead of importing Prime's session engine. Exact
publication details remain design work; no runtime dependency was added.

A temporary Python 3.13.12 loader probe accepted synthetic Prime-shaped packages
and exercised sync/async calls. Awaiting a synchronous run through Vivy's wrapper
raised TypeError, unlike Prime's always-awaitable wrapper; callable signatures
also differ. This was not a package-install, Prime runtime or model/evolution
acceptance run. Reference checkouts were unchanged; no product code was edited.

## Agent-led principle and transition to AutoDream design (2026-10-31)

The owner prioritizes agent freedom and More, Fast, Good, Frugal. The agent
chooses intentions and actions; the harness supplies reliable execution,
persistence, recovery, wake delivery, cancellation and configured resource
limits. Avoid embedding prescribed behavior or activity-specific policy engines
in the core. See DESIGN's agent-led principle.

Discussion item 3, persistent sleep/wake, is settled at the architectural
direction level. Implementation, exact integration details and runtime
acceptance remain pending; this checkpoint is not a delivery claim. Move the
discussion to item 4, AutoDream. Its existing decision already selects a
dedicated workspace, a separate persona-free Dream context, artifact/persona
candidates and automatic adoption without routine per-candidate human approval.
Refine triggers, collection, effective-version handling and outcome feedback
under the minimal-execution principle. The textual first-slice scope in the
detailed proposal is still a proposal. No AutoDream implementation starts with
this record; official inbox/general hooks remain deferred.

## Idle autonomy default and digital-resident research (2026-10-31)

The owner selects idle autonomous activity off by default, enabled on demand,
and identifies a game-world digital resident as a possible key delivery this
phase. Read the [resident research](research/2026-10-25-hybrid-agent-workbench-decisions.md#24-digital-resident-research-and-idle-autonomy-default-2026-10-31)
before settling scheduling behavior. Independent Minecraft multiplayer remains
the first game route; exact acceptance scope, edition/version and backend remain
open. The owner selects a companion who adventures with them and also inhabits
the world. If the owner logs out while the server remains online, the agent
decides whether to stay within the enabled game activity. This does not enable
global idle autonomy or override explicit stop/pause; the Vivy host must remain
available for continued activity. See DESIGN's companion-behavior clarification.
The research recommends evaluating `aibengineering/mine-ai-mcp` as an existing
action/reflex runtime behind the PEN, retaining Vivy as the only assistant
decision/persona/memory owner. Mindcraft and the cited papers provide additional
behavior and controller references. Treat active residency, background actions,
short tool boundaries and emergency body control as explicit design questions;
the official inbox and general hooks remain deferred. No game dependency was
installed, source cloned, server started or gameplay/provider test run.

## Recap ownership and Rig reuse accepted (2026-10-31)

The owner accepts the [Recap responsibility split](DESIGN.md#recap-ownership-and-rig-reuse-owner-2026-10-31).
Reuse Rig's existing summarization/compaction infrastructure. Rust owns formal
context composition, budgets, tool pairing, effective Recap and its publication
and recovery. Add only the assistant-specific summary policy, update triggers
and source checks that existing Rig facilities do not supply. The current
recipe already enables `CompactionPlugin`; this decision does not add another
summary engine or outsource context authority to Python.

Keep MemPalace and optional source/experience preparation in the Python
workbench. A future Python summary candidate must be checked and adopted by
Rust; no new bridge or parallel Recap database is required now. Laputa and
AutoDream ownership remain as previously accepted. Exact summary trigger/budget
settings and cross-session subject projection remain open; the later clarification
selects Recap-only history for assistant new turns. This is a documentation decision
backed by source inspection, with no implementation, dependency upgrade or new runtime
verification. The detailed proposal below remains subject to this decision.

## Rig reuse and upstream contribution assessment (2026-10-31)

The owner emphasizes direct Rig reuse and upstream contributions to reduce
downstream divergence. Read the
[current upstream assessment](research/2026-10-27-session-storage.md#current-upstream-adoption-assessment-2026-10-31)
and DESIGN's amendment before implementing the detailed proposal below. Keep
Plan B, but prefer existing Rig primitives and small upstream improvements;
the proposed private framed `DurableStore` is a fallback, not the first task.

Latest inspected main is `fc556f982d501eea9d7167398a19386e4884e776`, 12 commits
ahead of our unchanged `e1cd47d57` recipe pin. PR #2943 changes default recovery:
exit/restart stops unfinished turns, while public `CarryOn` explicitly enables
plugin-owned continuation. Saved state, `Interrupted`, holds and `WakeAt` plus
upstream corpus examples are reuse candidates. Ordinary log append still lacks
disk sync; a continuation marker alone does not satisfy the complete contract.
The latest stable release is v0.45.0, older than our current pin. A candidate
upgrade needs recovery, cancellation and TUI/plugin compatibility verification.
No dependency/reference-checkout update, issue/PR submission or runtime test is
delivered by this source audit. The preceding architecture's acceptance goals
and deferred real-provider gate remain unchanged.

## Detailed Plan B architecture proposed (2026-10-31, historical checkpoint)

The owner requested detailed architecture after selecting Plan B. Read the
[detailed baseline](DESIGN.md#preliminary-detailed-architecture-baseline-2026-10-31)
within the existing decision document. This entry describes the original
proposal; the consolidated baseline and owner rulings above supersede its
proposal status, custom-Store-first implication, mandatory sync, coding-delegation
comparison and text-only evolution scope.

The proposed minimum adds a `vivy-durable` plugin to the existing Rig execution
path. Reuse Rig's logical conversation logs and saved components; a framed
JSONL `DurableStore` synchronizes each append and exposes precise receipts.
Holds gate admission, dispatch, result settlement and continuation. Task state
and recovery data share an agent-log commit. The first path serializes tools
per agent to meet single-tool handoff and keeps unknown external outcomes
explicit. A saved one-shot wake needs a running host for on-time execution.

Later slices connect native Laputa, agent-owned Recap, existing MemPalace and
AutoDream. Persona/context updates become effective at a new model operation;
an interrupted operation retains its pinned context. The original text-only
AutoDream proposal was later replaced by persona and executable Skill closure
under AD-2. Product UI,
official inbox, external adapters and background-service deployment stay outside
the first infrastructure loop.

Source inspection used the recipe's Rig pin
`e1cd47d57f0bccb50ddffd6f45de2ac3365b271c`, not reference checkout HEAD. Required
proof remains: holds surviving multi-pass execution until `Last/WriteLogs`,
receipt/state matching, final redacted outcome capture, model-result recovery,
cancel ordering and fail-closed durable-format detection on resume. The private
flush/drain APIs mean a Store wrapper alone is insufficient. If the pinned public
seams cannot support the contract, identify the smallest required Rig API change
before expanding implementation; do not mutate the reference or silently change
architecture. D0 verifies these seams through one coding task; D1-D3 close
execution, context/memory and wake, then D4 evolution. See the proposal's
acceptance table. No product code or new runtime/provider verification is
delivered by this document update.

## Plan B selected: extend the existing execution chain (2026-10-31)

The owner selects DESIGN's
[Plan B](DESIGN.md#plan-b-durable-execution-within-the-existing-rig-chain-owner-2026-10-31).
Keep one Rust Vivy core and the existing Rig model/tool execution path; add
persistent phases, commit boundaries, recovery, cancellation and sleep/wake
semantics, using the current coding face for validation. Pi durable is a full
agent execution runtime designed for persistence and is the semantic anchor,
not a selected TypeScript dependency or a CLI mode to enable.

Mysoul informs Recap, explicit recall, autonomous wake and silence. Preserve the
accepted Laputa/persona, agent-owned task context, native MemPalace/workbench and
AutoDream boundaries. JSON/JSONL remains the host storage direction. Product
navigation and the official inbox schedule remain parked as recorded below.

Next detail the minimal state/commit contract and verify the Rig seams for tool
start, result publication, restart and cancellation. Existing Rig log flushes
do not alone prove durable transactions or disk sync. Begin with one recoverable
model/tool task, then context/memory and wake integration. Record remaining seam
limitations explicitly; do not silently replace the selected route with coarse
task continuation or a new execution loop. No implementation or new runtime
verification is delivered by this decision commit.

## Current priority: one Vivy core, validate through Coding (2026-10-31)

Read DESIGN's
[single-core decision](DESIGN.md#single-vivy-core-and-infrastructure-first-decision-owner-2026-10-31)
first. The owner settles one Vivy technical foundation and prioritizes completing
agent infrastructure using the existing coding face for actual validation.
Reuse the working Rust components, with Pi durable as the persistent-lifecycle
reference and the accepted Laputa, Recap, MemPalace and AutoDream boundaries.
Keep workload-specific state and recovery contracts explicit in this core.

The role entry, session-menu layout, independent-session navigation, assistant-
first delegation and Diva desktop/pet/bundle form remain **undecided and parked**.
The owner explicitly withdrew those topics from the immediate decision scope.
Do not treat the earlier conversational confirmation as an approved UI design,
inspect the old Vivy/Diva projects, or make a new interface a prerequisite.

Next identify the infrastructure gaps and define the smallest behavior to build
and verify through the current coding face. This decision delivers no runtime
code or new test evidence; outstanding provider acceptance remains recorded,
and the official inbox implementation remains deferred.

## Durable design preparation and Pi refresh (2026-10-31)

The owner now explicitly starts persistent-assistant design preparation, with
pi-durable as the anchor and the mysoul basic loop, Laputa governance and
AutoDream decisions as inputs. The owner confirmed that "myllm" in the request
means mysoul. This supersedes the older instruction to defer durable research;
the current request is inspection followed by discussion, not implementation
approval or a new real-provider acceptance result.

The clean `.workspace/pi` reference was fetched and moved in detached HEAD from
`f321e5bdd26b3624bfb2ab182a334d0a5edb698a` to upstream `main` at
`a14cdf90f0a76532f3e20bc3c01872837cef24ba` (commit date 2026-10-30).
No dependencies were installed and no Pi code was modified. Earlier coding
comparisons remain evidence for their original revision. Read the refresh in
[session storage research](research/2026-10-27-session-storage.md#durable-reference-refresh-2026-10-31)
before relying on the old durable storage or notification descriptions.

Key differences for discussion: Pi steering follows a whole tool round, whereas
the owner selected a single-tool handoff boundary; Pi's new `afterRun` work keeps
the conversation busy and cannot be assumed to be the AutoDream scheduling
policy; streamed progress can now be visible before persistence, unlike task
admission and tool intent. The official inbox remains deferred. Rust lifecycle,
storage, context refresh and recovery details still need a bounded design.

Source and existing verification records were inspected in mysoul's current
Desktop checkout (`3a7cbde7`), including Recap-only context, explicit recall and
autonomous wake scheduling. No application was run or live data accessed. The
previous Vivy workbench provider gate is not retrospectively marked passed.

## Laputa protocol and native governance checkpoint (2026-10-31)

Read DESIGN's
[Laputa protocol and Garden governance decision](DESIGN.md#laputa-protocol-and-garden-governance-decision-owner-2026-10-31)
before continuing persona, memory or evolution design. The owner repositions
Laputa as a specification/protocol with host-specific implementations. Vivy's
preferred direction is a small native Rust persona component with read/update
operations and top-level persona-context injection. A complete Garden/Go runtime
or external service is not required. Exact APIs and persona format remain open.

The agent/harness owns Recap/ACTMEM. Full Python-native MemPalace remains the
current long-term engine; later backend replacement, such as Mem0, is an
architecture goal, not an implemented adapter or migration. Do not duplicate
persona in Prime harness entries or inject MemPalace L0.

Garden remains an optional reusable governance workflow for other agents: collect
corpus, run a selected agent, apply updates to the host's existing authority,
verify effective changes and retain outcomes. Respect native files/evolution
instead of establishing parallel persona or Recap stores. Vivy AutoDream retains
automatic candidate adoption; other hosts keep their own write policies. Hermes
and OpenClaw are intended adapter examples, not verified integrations. A Web
console and Garden control of Vivy's internal Laputa remain later discussion.

This direction is synchronized to the local Garden/Laputa project's ADR-0016.
Documentation only: no new runtime, backend, adapter, durable work or inbox work
is delivered; the owner-deferred real-provider workbench gate remains unchanged.

## Inbox scheduling checkpoint (2026-10-31)

The owner defers the official inbox plugin and its notification/hook and durable-
agent integration to a later phase, outside the immediate implementation stage.
Keep the accepted behavior in DESIGN's
[Inbox and event handoff decision](DESIGN.md#inbox-and-event-handoff-decision-owner-2026-10-31),
but do not start it or make it a nearest-stage gate. Pending messages survive
restarts; the agent chooses reading/deletion/reply and controls reminder hooks.
Ordinary hook/chat/new-task handoff waits for one tool execution to finish, then
the agent decides what to do; humans retain explicit immediate interruption.
No implementation date, exact later-phase order or full swarm runtime is selected.

## PEN, NeuroLink and Minecraft decision checkpoint (2026-10-30)

The owner locks the positioning in DESIGN's
[PEN and NeuroLink decision](DESIGN.md#pen-and-neurolink-decision-owner-2026-10-30).
PEN packages deliver complete external capabilities on import and may combine
tools, MCP connections, Skills, connector code, resources and configuration.
PEN is broader than an MCP configuration bundle. NeuroLink is a separate
A2A-like protocol/standard for external conversations and events; channels are
one integration class. Execution and actual lifecycles remain in Vivy or the
responsible PEN module, not in a protocol-level attention/scheduling system.

Minecraft is the first selected game. Independent-player multiplayer is the
primary route; computer-controlled client play must remain possible in the
design but is not a second first-delivery gate. Edition/version, Mineflayer or
another connector, auxiliary-mod policy, manifest/runners, hub distribution and
the exact A2A/ACP/AG-UI relationship are not finalized. Mineflayer remains a game
connector candidate, not a verified Vivy integration.

Final channel ruling, 2026-10-30: port/adapt channel packages from
[Octos](https://github.com/octos-org/octos) and load them through NeuroLink.
Octos replaces the earlier ZeroClaw candidate. The inspected channel boundary is
`crates/octos-bus` at `7486f3df54b5803ec4095c60b244617c9d254750`; that crate
also contains session/cron facilities beyond the selected port. Read DESIGN's
**Channel packages: port Octos and load through NeuroLink** for the source links
and scope. This selects channel reuse, not Octos's agent runtime or OUP. Package
extraction/loading details remain open; no port, build or runtime acceptance is
delivered. Owner refinement, 2026-10-31: only the official QQ Bot channel is
selected from Octos. NapCat QQ is a separate actively browsable PEN capability,
using NapCat/NTQQ's existing data rather than a second mirrored QQ database.
Read DESIGN's NapCat section before treating it as another passive channel.

This is a documentation-only decision checkpoint: no PEN/NeuroLink/Minecraft
runtime is delivered. Recipe-selected compilation, existing setup/trust rules,
durable sequencing and the deferred workbench provider gate are unchanged. Read
the locked decision before using older PEN-undefined or channel-only scope notes.

## AutoDream decision checkpoint (2026-10-29)

The owner selects AutoDream for the new Vivy's evolution chain: a collection
workflow supplies a dedicated evolution workspace; a dedicated Dream prompt
runs without Vivy's conversational persona, organizes the workspace and produces
Prime-style artifact candidates plus persona data. Candidate generation remains;
adoption is automatic, without routine human approval of each candidate. Do not
reduce the implementation to proposal generation awaiting manual activation.

Read DESIGN's **AutoDream evolution decision** before continuing evolution work.
It supersedes the older outside-Vivy/research-only direction and does not select
Prime's refine-trigger route or port the old AutoDream implementation. Exact
artifacts, triggers, validation, adoption/version boundaries and recovery remain
design work. Existing recipe compilation and initialization-consent boundaries
remain in force. This checkpoint records a decision only: AutoDream is not yet
implemented, and the deferred workbench provider gate and durable sequencing
are unchanged.

## Initialization closure (2026-10-28)

The approved plan is `docs/plans/2026-10-28-initialization.md`. Setup now registers
from compiled plugins, follows trust/provider/Python/optional memory, remembers
project skips and scoped installation consent in user state, and offers `/setup`.
Python startup detection, consented repair, managed Windows x64 uv/Python,
structured stages, fixed native memory validation and safe next-cell environment
adoption extend the existing workbench. Provider configuration has save-only and
explicit streaming/tool verification with cancellation and a deadline. MemPalace
remains opt-in. Do not restart durable or the deferred StepFun acceptance.

Verified during implementation: root workspace tests (18+1+2+1), basics 82,
gateway 13+1+6+14, Python Rust 20 and all 43 Python/native tests (44.403s).
The new Windows
`scripts/initialization_tui.py` passes
ordered prompts, remembered skips, manual preparation, approved repair after a
missing interpreter, no automatic provider requests, explicit streaming/tools
and pinned provider failure, including cancellation that preserves the earlier
error. A real isolated managed-toolchain download verified
the pinned uv archive and installed Python 3.13.12 without system registration.
The complete scripted Coding/Skill/MCP/native-memory workflow passes in 85.774s
(33.006s candidate preparation); it is not real-provider verification.

One independent whole-branch review found three Important issues, reproduced
and fixed: interpreter startup before scope validation, descendants retaining
installer pipes after parent exit, and an extra package index omitted from the
consent scope. Startup now checks both scope and a locally recorded publication
hash in Rust before executing a prepared interpreter. Legacy/copied environments
need `/setup python` or `/python prepare` once; automatic detection does not run
them. This local validation record is separate from future installation consent.

Windows initialization, workbench rotation/lifecycle, branding, Plan and provider
terminal regressions pass, including 120x40 and 50x20 provider/branding screens.
The headless workbench and recipe-omitted Python dependency checks pass. Twenty
real workbench/child/MCP lifecycle cycles pass with zero orphans; cold/warm median
calls were 0.835/0.066 seconds and maximum close plus confirmed exit was 0.164s.
Queued-call cancellation, compaction retention, crash/timeout recovery and Skill
removal also pass. The lifecycle harness now waits for compaction's own end event
before submitting its warm-state check.
Formatting, PowerShell parsing and diff checks pass. One preceding Python run
hit a Windows temporary-venv cleanup sharing error after its assertions; no
fixture process remained, and the complete suite passed using a dedicated temp
directory. Keep that environmental failure distinct from functional acceptance.

After fast-forward integration, the main-checkout launcher and Coding binary were
rebuilt. Root tests (22), the original launcher Coding smoke and the complete
initialization terminal scenario pass on main, including automatic repair,
explicit provider streaming/tools and cancellation preserving a prior failure.
For another machine: pull main, run `build.cmd` then `start.cmd`, and follow the
startup guide or `/setup`. MemPalace remains an explicit optional setup item.

## Footer rotation (2026-10-28)

The terminal alternates right-side telemetry and Python/memory status every five
seconds, pinning preparation/errors and holding recovery for five seconds.
The existing runtime publishes per-agent observations; the display uses Rig's
clock and fitting without launching Python, installing packages or querying the
database. `/python status` refreshes the cached interpreter check. See DESIGN's
footer section and `scripts/workbench_tui.py` for behavior and verification.

## Active workbench checkpoint (2026-10-28)

Implementation is integrated into `main`, including the remote one-click build
and terminal scroll/resize changes. The ignored `.workspace/python-workbench`
worktree retains local acceptance environments and evidence. The earlier
2026-10-27 sections below are historical: the new
owned Python runtime, Rust dispatcher, explicit environment preparation, trusted
Skill loading, generic MCP, complete artifacts and native MemPalace facade now
exist. Read DESIGN's latest checkpoint, README's workbench usage and the plan
before continuing. Durable has not started.

Verification commands belong to this repository: root `cargo test --workspace`,
focused Cargo tests for `plugins/vivy-python` and `plugins/vivy-basics`, and
`python -m unittest discover -s plugins/vivy-python/tests` using the prepared
interpreter (with locked MCP/native dependencies installed). Native tests require
an explicitly prepared isolated root in `VIVY_MEMORY_TEST_ROOT`; without it they
are reported as skipped. The real memory acceptance venv and assets are ignored
under the implementation worktree's `.workspace/memory-acceptance`.

The new `scripts/workbench_smoke.py`, `workbench_lifecycle.py`,
`workbench_optional.py` and `workbench_tui.py` provide Coding, 20-cycle ownership,
recipe omission/headless and Windows terminal checks. The joint script's default
gateway is scripted; `--real` uses an explicitly configured endpoint and credential
and must never be reported as passing based on its default mode. Keep test data
synthetic and separate from any existing user's palace or session directories.

The independent review found five Important issues; all were reproduced and fixed.
Final verification passes 13 Rust workbench tests and 36 Python tests including
9 native tests, actual queued-call close/reset, compaction retention, 20-cycle
ownership, Windows Python terminal and original Coding smoke. Maximum close command
plus process-exit confirmation is 0.256 seconds. Complete scripted joint acceptance
now independently inspects artifacts and reopens memory; it passes in 84.28 seconds.

Merged-main verification also passes root workspace tests, 33 gateway tests,
78 basics tests, 13 Rust workbench tests, and all 36 Python/native tests.
The rebuilt main-checkout launcher and Coding binary pass the Windows branding
terminal at 120x40 and 50x20 (including resize), Python terminal lifecycle and
original Coding smoke. Formatting, PowerShell parsing and diff checks pass.
The optional Desktop dialect enters the same first-run provider bootstrap and
is covered for persistence and preservation of existing managed settings.

For another machine, pull `main`, build with `build.cmd`, launch with `start.cmd`,
then run `/python prepare` (including native memory preparation as described in
README). Local virtual environments, model caches and credentials are not shipped.

Outstanding: the owner deferred actual-provider verification to a later manual
test while changing provider. Stop StepFun retries. Earlier attempts failed TLS
before receiving a model response. The authorized credential was not committed or
printed. Configure the replacement through the existing provider workflow, use an
isolated synthetic test root, then run the joint scenario. `--real` takes an
OpenAI-compatible endpoint via `OPENAI_BASE_URL`, an explicit `VIVY_TEST_MODEL`
and `RIG_SECRET_GATEWAY`. For another compiled protocol, perform the same recorded
workflow through that provider's existing configuration. Do not
claim M3 complete or start durable before that system gate is accepted.

## Goal and current state

The owner wants a fast Rust experiment preserving Vivy's useful idea of a species
assembled from a YAML recipe at compile time. Rig supplies the current coding
implementation. Model access is gateway first. The latest 2026-10-31 decision
settles one Vivy core: complete agent infrastructure and validate it through the
existing coding face. Classic Pi remains the coding reference and pi-durable
anchors persistent lifecycle behavior within that shared technical foundation.
Product entry points and Diva packaging are deferred. Coding has passed initial
real-task acceptance; the new durable behavior still needs its own execution
evidence. The delivered Python workbench and native MemPalace integration remain
available, with their deferred real-provider acceptance recorded above.
Both paths use the same workspace and may recall the same domain-partitioned
MemPalace memory. The host's Laputa implementation owns persona; do not inject
MemPalace's L0 identity. The agent/harness owns Recap/ACTMEM; durable's layered
context remains a later memory slice. Do not apply mysoul's companionship context
policy to coding by default. Domain wings,
topic rooms and source drawers are distinct from the optional L0-L3 context
stack. Preserve per-task state/namespace boundaries within the shared workspace.
Recursion, factory, a complete Prime daemon and automatic evolution are not
prerequisites. Read DESIGN's **Single Vivy core and infrastructure-first decision**
before the historical two-harness discussion. Durable infrastructure is not yet
delivered; this decision does not change that implementation status.

### Saved discussion checkpoint (2026-10-27)

Latest owner acceptance (2026-10-27): the agent completed a real coding task by
fixing one of its own bugs. The owner confirmed the chain was stable and
explicitly accepted it. The initial real-task coding gate is closed. The owner's
subsequent sequencing decision makes the Python workbench the next deliverable,
with durable starting after workbench system validation. This is owner-run
acceptance, separate from the historical automated
checks. Do not require another coding demonstration before that next slice.

The owner asked to save today's decisions before further work. Resume from
DESIGN's current two-harness section and **Python-native memory candidate and
research checkpoint**, not the older full Prime/evolution direction. The order
is now: accepted coding foundation, new Python workbench design and system
validation, then the separate durable foundation. Shared workspace/domain memory does
not merge their task state or replay semantics. Keep the implementation small.

The latest investigation is a native MemPalace library facade inside Python.
Programmatic search and collection access exist, but native write/chunk/read
equivalence is unverified. remember/search/get are proposed facade names, not
shipped APIs. Laputa supplies persona; MemPalace L0 is not injected. No MemPalace
fork, native-memory implementation or dependency change was made. Record costs
before treating optional-feature omission as a smaller installation/runtime.

Prime runtime `c4c238ca025e382d96fb5f37e6328ca77fb82ef3` passed seven isolated
Python protocol/lifecycle probe checks; that does not validate its Rust adapter,
MCP, snapshots or full background operation. Existing vivy worker tests passed
six cases; its asyncio tasks are still cancelled at cell end. DESIGN preserves
the exact inspected paths, pins, probe limits and mysoul reference locations.
mysoul remains read-only reference material; no sessions or memory data are
copied. Next work requires a bounded implementation scope, not another broad
capability inventory. Today's save contains documentation only.

Repository: `https://github.com/mastwet/vivy-test`.
The owner's subsequent Go/remake migration assessment is recorded in
[research section 18](research/2026-10-25-hybrid-agent-workbench-decisions.md#18-go-vivy-migration-assessment).
It checks pinned source sizes and donor overlap, proposes a minimum behavior
selection, and gives a conditional engineering allowance. It does not authorize
implementation or change the two-harness scope. Original source was inspected
read-only; no app data, source port or legacy data migration was involved.

The owner's next refinement reduces migration scope further: retire BML, use
the full upstream MemPalace engine in Python (still no L0 persona injection),
and rewrite Laputa. Small bundles and hooks remain intended capabilities.
Plan is implemented; external hook and saved session-goal examples exist in Rig
but are not selected. Autonomous Goal remains unwired. See DESIGN's native-memory
section and research 18.7; do not reuse the earlier broad estimate as a quote
for copying old features.

The owner requested a broad feature-by-feature reconciliation, including A2A.
[The Vivy feature inventory](research/2026-10-27-vivy-feature-inventory.md)
separates owner scope decisions from actual target implementation, covers all
16 remake lanes plus A2A/ACP/SSH and lists unresolved product choices. A2A has
accepted server source/evidence on its Go feature branch but is not selected in
vivy-test; its server and an outbound A2A client are separate capabilities.
Use that inventory rather than treating older suggestions to defer as decisions.

Latest owner decisions narrow the inventory: no legacy data or deployment
compatibility; no old Notebook/Reports, BML, Skill-evolution or AutoDream
migration. A new MemPalace + Obsidian loop is intended for the Python workbench
and awaits design; new Prime-style evolution is outside Vivy. INOFY-style
workflows, if selected, belong in Python. Cron/webhook/background are redesigned;
LSP is optional; QQ channel is the only first-phase channel candidate and remains
undecided. ACP/SSH use references rather than the old Go architecture. The
inventory also records the Prime and Rig history/context/file-rewind source
findings; no such additional implementation is implied.

The later [session storage comparison](research/2026-10-27-session-storage.md)
records Prime, Pi coding, pi-durable and the current Rig storage boundaries. It
records the owner's later preference for JSON-based files, retaining Rig's
coding store and evaluating a Rust-owned file-backed durable store. The durable
format and recovery protocol remain open and are not implemented.
Prime's fast window path falls back on Windows; pi-durable's Node SQLite defaults
and single-owner assumption are explicitly qualified in that research.

The owner further clarified: do not add a Vivy-owned SQLite database alongside
MemPalace; focus on a Python-workbench ingestion and recall flow into MemPalace's
own storage. JSON-based files remain the proposed session/task substrate.
Single-binary deployment is not important. This is not a ban on the storage
technologies internal to MemPalace. Ingestion policy and durable file commit
semantics are not yet designed or implemented; see DESIGN.

Current state: the builder, gateway, native Windows launcher, Git Bash process
adapter and auxiliary Python workbench are implemented. DESIGN's "Windows coding
species and Python workbench" section records implementation evidence. The owner
accepted the real-task coding path as recorded above. Gateway model facts
(delivery step 1) are implemented and checked against
the scripted gateway. DESIGN's 2026-10-27 two-path decision supersedes conflicting
scope and ordering in its older "Delivery slices and stopping points" record.
The later owner-approved collaborative Plan addition is implemented in
`vivy-basics::plan::PlanPlugin` and selected in the baseline recipe. It includes
terminal mode markers, explicit plan submission and user review/feedback;
see DESIGN's "Collaborative Plan mode" and README usage. Goal and automatic
evolution remain separate work.
The owner-approved 2026-10-26 terminal refresh selects a compact vivy-test screen
on Rig's public widgets, a persistent footer and a branded default identity.
See DESIGN's "Compact vivy-test terminal" and the verification appendix below.
Read [DESIGN.md](DESIGN.md) for decisions and [PI-BASELINE.md](PI-BASELINE.md) for
coverage. Those documents replace the need to reconstruct the chat history.

Latest delivery (2026-10-25, `feat/subagents`): the recipe selects a Vivy wrapper
around Rig's native delegation. It provides roles, task/message/wait, bounded
parallel batches, chains, saved child follow-up and terminal progress/usage.
Children share the host process and project context; cwd stays within the project.
Default depth is one, with four active children per session. No Python bridge,
apixis, sandbox or automatic worktree was added. See DESIGN and
[delivery evidence](logs/2026-10-25-subagents.md); mirrored research section 17.7
supersedes the previous single-child scope. Commit c94f87f was fast-forward merged
into local main on 2026-10-25. Collaborative Plan commit b43d448 is integrated
alongside it on 2026-10-26. This integration has not been pushed.

## Reference setup

Clone **Pi and Rig themselves**, including source, into this checkout's ignored
`.workspace/`. Installed packages alone are insufficient for the investigation.
Do not copy old reference folders from the original Vivy checkout. Do not clone
Vivy: it is undergoing separate repair, and this repository must not inherit its
build, runtime, SDK or working-process dependencies.

Run from the `vivy-test` root, only for reference directories that do not already
exist. If a checkout already exists, inspect its status and remote before making
changes; do not overwrite somebody's work.

```sh
git clone --depth 1 https://github.com/earendil-works/pi.git .workspace/pi
git -C .workspace/pi fetch --depth 1 origin a14cdf90f0a76532f3e20bc3c01872837cef24ba
git -C .workspace/pi checkout --detach a14cdf90f0a76532f3e20bc3c01872837cef24ba

git clone --depth 1 https://github.com/0xPlaygrounds/rig.git .workspace/rig
git -C .workspace/rig fetch --depth 1 origin e1cd47d57f0bccb50ddffd6f45de2ac3365b271c
git -C .workspace/rig checkout --detach e1cd47d57f0bccb50ddffd6f45de2ac3365b271c
```

These ordinary source pins identify what was inspected. They do not introduce a
species hash system. If a revision cannot be fetched, report it; do not silently
substitute current HEAD and repeat the old findings. Updating the baseline is a
normal dependency decision after reviewing the relevant changes.

The commands above are setup instructions when references are missing. No upstream dependencies need
installation just to read source. Read each reference repository's own
instructions before running its tooling. Keep product changes in `vivy-test`.

## Where to look first

| Question | Source in the reference checkouts |
| --- | --- |
| How does Rig already generate an agent? | `rig/src/launcher/project.rs`, `build.rs`, `rig/crates/rig-harness/README.md` |
| What can the existing product reuse? | `rig/plugins/rig-basics/`, `rig/plugins/rig-tui/`, `rig/crates/rig-harness/PLUGINS.md` |
| What is the smaller library route? | `rig/crates/rig-agent/README.md`, its `src/run/` implementation, `rig/crates/rig-core/` |
| What do coding tools and session storage actually guarantee? | `rig/crates/rig-tools/`, `rig/crates/rig-memory/`, `rig/crates/rig-ecs/src/stage.rs` |
| What does Pi's loop do? | `pi/packages/agent/src/agent-loop.ts`, `pi/packages/coding-agent/src/core/agent-session.ts` |
| What must session and extension behavior preserve? | Pi coding-agent `docs/sessions.md`, `session-format.md`, `compaction.md`, `extensions.md` |
| What are the current MCP and Code Mode capabilities? | Pi coding-agent `docs/mcp.md`, `codemode.md`; Rig `crates/rig-rmcp/`, `plugins/rig-steel/` |

## Suggested next implementation task

Owner decision, 2026-10-27: the workbench closure scope, exclusions and transplant/
local implementation ownership in [research section 19.9](research/2026-10-25-hybrid-agent-workbench-decisions.md#199-closure-scope-and-implementation-ownership-2026-10-27)
are accepted and recorded in DESIGN. Do not reopen the broad scope decision.
Next turn the accepted boundary into concrete adapter contracts and an
implementation plan. The sequence is core W1-W5 validation through Coding,
then the bounded native MemPalace consumer gate, then durable. Scope acceptance
is not runtime acceptance: sixteen upstream REPL tests passed on Windows, but
MCP transport and the proposed Vivy adapter were not run.

The 2026-10-28 deeper Prime donor audit is in research section 19.10. It adds
protocol/output attribution and loss boundaries, startup/cancellation races,
environment locking and package metadata, MCP pagination/credential/lifecycle
concerns, a component-by-component extraction map and focused verification
cases. In particular, upstream `repr` and current Vivy collection can truncate
before a spill, so the accepted complete-artifact contract needs producer-side
design. This is source inspection only; no new runtime result or scope change.

Owner focus, 2026-10-28: research and close the workbench plus bounded native
MemPalace consumer; do not start durable research now. Research section 19.11
maps the pinned MemPalace library seam, source-identity collision in the MCP
content-derived ID, chunk/get and backend cleanup boundaries, Windows writer
contention, embedding preparation and blocking-call cancellation. It proposes
synthetic acceptance cases and records the installed/pinned version mismatch.
No MemPalace runtime test or product implementation was performed.

The subsequent component audit is in research section 19.7. It identifies the
REPL/protocol/bridge, executable-Skill loading and generic MCP lifecycle as donor
candidates, and maps all nine bundled Python Skills to their actual host or
library dependencies. The current Vivy worker protocol is incompatible with a
drop-in Prime worker; host routing must change with it. Keep `vivy-process` and
existing direct tools. No transplant has been implemented or validated yet.

The owner supports this selective-reuse direction and requested investigation of
Python-side tool loading. Section 19.8 records the source findings and a bounded
loading contract. Resolve that contract in the design: installed packages,
bound Python names and model-visible capabilities are distinct; Skill code
updates and MCP connection reload have different lifecycles. Extend W2 with
namespace conflicts, unavailable capabilities and update/removal behavior.
The accepted restart policy is fresh Python state plus preserved explicit
artifacts and a reset notice, without automatic cell replay. Skill code updates
use a worker restart boundary; MCP connection reload is separate. The native
memory consumer remains a separate gate from W1-W5 and the durable lifecycle.

The 2026-10-27 scope clarification takes precedence over older sequencing below.
The classic-Pi coding path has passed owner-run real-task acceptance. Prioritize
the new Python workbench: close its design, implement the selected runtime and
adapters, and validate the complete capability lifecycle independently and
through coding before starting durable. Preserve the working coding path and
its existing regression checks. Current worker code preserves variables but
cancels async tasks at cell end; timeout/cancellation resets its process. The
earlier Prime probes do not establish a delivered replacement.

Define bounded acceptance scenarios for discovery/instructions, persistent
resources, Python/MCP calls, correlated bounded results, failure, cancellation,
reset and cleanup. Exact extraction boundaries, adapter contracts and test
fixtures remain design work within the accepted scope. Keep future host
integration explicit without adding a universal harness or moving authoritative
task state into Python.

After workbench validation, establish the pi-durable-referenced Rust harness
and test its workbench integration under its own recovery/replay rules.
`pi-durable` is a TypeScript reference, not a Rust integration already present.
mysoul's memory behavior remains a durable-path reference, not a dependency on
its application, private data or runtime.

The owner also requested provider-management research on 2026-10-26. See
[provider management and plugin boundaries](research/2026-10-26-provider-management.md)
for Pi/DSH/Rig evidence and a proposed extension of the gateway-first surface.
The owner fixes provider scope to OpenAI-compatible and Anthropic-compatible
protocols only, with built-in presets and user-defined instances derived from
them. Subscription login/OAuth/token refresh and other native protocols are
excluded. DESIGN records this scope; the research records Rig's registration
boundary and an offline API-construction probe, with no live-provider claim.
Provider management commit `de10ee3` was fast-forward merged into local `main`
in `vivy-test` on 2026-10-26. The baseline selects ProvidersPlugin, two protocol
features and the full TUI editor. README documents commands and configuration;
the existing research document records verification and review corrections.
The native launcher and baseline agent were rebuilt from this main checkout.
The integration remains local. No new live-provider or non-Windows platform
claim is made.

The older coding-only delivery sequence and its verification gaps remain in
DESIGN and PI-BASELINE. Preserve the Windows launcher and Rig harness/TUI while
checking that path; keep actual gateway evidence distinct from mock checks.
Those coding records do not establish or select the durable implementation.
The validated Python workbench now precedes the durable implementation.
Automatic evolution and PEN/NeuroLink have later dated decisions above;
their implementation and persona growth are not delivered by this checkpoint.

Use existing authorized gateway configuration without printing credentials. If
essential configuration is unavailable, request only that missing information.
Do not guess secrets or present a scripted endpoint as a real gateway smoke.

## Research directions and traps

- **Measure iteration cost.** More upstream reuse can mean more dependencies but
  less owned product code. Check cold/warm/plugin-edit builds and debugging cost
  before deciding that a smaller-looking crate is the faster foundation.
- **Prove real optionality.** Inspect the resolved dependency graph for each
  generated agent. Shared upstream crates can contain several tools/providers;
  omitting a wrapper does not prove all their source was excluded.
- **Keep feedback useful.** Preserve bounded task evidence for useful errors and
  future memory/refinement work. The Prime workbench loop does not require its
  incident CLI, daemon, service catalog, subagent platform or automatic evolution.
- **Preserve semantics as coverage expands.** Branch-aware state, cancellation,
  extension interception, and tool-result pairing matter more than matching
  command names. Use representative behavior checks.
- **Keep source assertions modest.** Rig library availability does not prove
  integration with this gateway, terminal, platform, or recipe. New APIs at this
  pre-release revision may differ from older online examples.

## Evidence at handoff

DESIGN's dated implementation sections retain build, scripted-gateway and Windows
smoke evidence and limitations. They do not verify the new hybrid design. For the
2026-10-25 documentation delivery, review content/links and run `git diff --check`;
runtime CI is unnecessary because no executable behavior changed. Later code
deliveries must use the repository's focused checks and a representative real-path
smoke. Source availability and a successful manual run do not prove full parity.

## Collaborative Plan verification (2026-10-25, Windows)

The collaborative Plan delivery supersedes the documentation-only scope above.
Checks used Rust 1.97.1, `CARGO_TARGET_DIR=C:\vt` and
`CARGO_PROFILE_DEV_DEBUG=0`:

```powershell
cargo test --workspace --locked
cargo test --manifest-path plugins/vivy-basics/Cargo.toml --features tui --locked
cargo check --manifest-path plugins/vivy-basics/Cargo.toml --lib --no-default-features --locked
cargo run -p vivy -- build recipes/coding-agent.yaml
python scripts/smoke.py C:/vt/debug/vivy.exe
python scripts/plan_tui.py C:/vt/debug/vivy.exe
cargo fmt --manifest-path plugins/vivy-basics/Cargo.toml -- --check
git diff --check
```

Results: 20 workspace tests and 37 basics tests passed, including 14 Plan cases.
Plan cases cover unchanged tools and writable behavior, exact review/feedback,
cancellation, missing reviewer, stale approval, modal `/plan off`, running-turn
transitions, session restore and fork isolation. An unfinished review was also
reopened to verify it never replays approval. Core feature tests failed before
implementation; stale-approval and modal-command regressions failed before their
corrections, then passed.

The real Windows ConPTY terminal passed 16 scripted checkpoints: Plan entry and
ordinary clarification, mode/review markers, scrolling an 80-step plan, approval
followed by a real file write, feedback, Esc, manual exit inside the modal,
re-entry and process/session resume. Captured gateway requests confirmed guidance
changes and a stable tool catalog. The existing gateway/shell/session/Python/JSON/
error/secret-filtering/image/reasoning/compaction smoke passed. No live-model or
new macOS/Linux acceptance is claimed.

The terminal driver uses test-only `pyte==0.8.2`, `wcwidth==0.2.13` and, on
Windows, `pywinpty==3.1.0`; these are not agent/runtime dependencies. They were
installed in ignored scratch and exposed through `PYTHONPATH`. The Windows
driver selects UTF-8 before launch. Its optional evidence output contains only
synthetic test screens and requests and remains outside Git.

Two disposable recipes were built and exercised: without Plan, the model receives
no Plan tool/guidance; without the terminal, Plan submission stops with a clear
missing-reviewer message and `vivy-basics` has no normal `rig-tui` dependency.
Temporary recipe variants are not maintained configuration sources.

Fresh independent review found the modal command interception; the typed command
now routes through the same backend command and is covered in both unit and
terminal tests. Re-review reported no remaining substantiated findings. Debug
terminal startup also confirmed the explicit phase ordering removed the Plan/
compaction ambiguity. Existing Windows fixture assumptions in context/trust
tests and a smoke assertion assuming every JSON `text` field was a string were
corrected in test code; related product behavior was unchanged.
Formatting and Git whitespace checks passed. Clippy completed with one existing
`collapsible_if` suggestion in the Windows shell implementation, outside this change.

## Combined Plan and subagent integration (2026-10-26)

The owner approved merging collaborative Plan commit b43d448 into local main,
which already contains subagent implementation c94f87f. Conflicts were limited
to README composition, plugin documentation and a trust-fixture comment. Both
plugin exports and recipe components are retained; the later empty-discovery
trust correction remains intact. Plan retains its advisory per-agent semantics
and does not introduce another permission or agent execution path.

Integration verification used CARGO_TARGET_DIR=C:/vt and
CARGO_PROFILE_DEV_DEBUG=0. The root workspace's 20 tests and the basics plugin's
61 tui-enabled tests passed, including a new combined native-stage-order check.
The no-default-features library check, root/basics formatting and Git whitespace
checks passed. The baseline recipe rebuilt successfully for the existing start.cmd.

Both scripts/plan_tui.py and scripts/subagents_smoke.py --tui passed on the actual
Windows ConPTY terminal. Checks included Plan approval followed by a real write,
feedback, modal exit, restart/resume, parallel/chain delegation, role trust,
usage, and cancellation of four shell descendants without starting queued tasks.
The existing scripts/smoke.py passed gateway, shell, session, Python, JSON,
failure, credentials and model-facts checks. Test dependencies were reused from
ignored scratch; none were added to the product.

The Plan terminal script initially hit stale slash-command completion at different
steps. Outside a review, it now waits for the exact editor text, dismisses
completion and then submits; the full scenario passed. Modal command behavior
and product keybindings were unchanged. This scripted gateway verification does
not establish real-provider compatibility. No push was performed.

## Compact terminal verification (2026-10-26, Windows)

The recipe now selects `VivyPromptPlugin` and the feature-gated `VivyTuiPlugin`.
The local Codex source at `970111f6cb` was inspected for presentation; neither
reference checkout was edited or made a new build/runtime dependency. Rig's
native execution, session, question, editor and transcript paths are retained.

Checks used the pinned Rust 1.97.1 toolchain, `CARGO_TARGET_DIR=C:/vt` and
`CARGO_PROFILE_DEV_DEBUG=0`:

```powershell
cargo test --workspace --locked
cargo test --manifest-path plugins/vivy-basics/Cargo.toml --features tui --locked
cargo check --manifest-path plugins/vivy-basics/Cargo.toml --lib --no-default-features --locked
cargo run -p vivy -- build recipes/coding-agent.yaml
cargo fmt --all -- --check
cargo fmt --manifest-path plugins/vivy-basics/Cargo.toml -- --check
git diff --check
python scripts/branding_tui.py C:/vt/debug/vivy.exe
python scripts/plan_tui.py C:/vt/debug/vivy.exe
python scripts/subagents_smoke.py C:/vt/debug/vivy.exe --tui
python scripts/smoke.py C:/vt/debug/vivy.exe
```

The 20 root tests and 73 tui-enabled basics tests passed. The new tests cover
model-visible default identity and preservation of an explicit role, changing
welcome metadata, Unicode and resize fitting, persistent draft-time status,
NO_COLOR, dark/light input surfaces with preserved diff/failure colors, native
multiline Paste events, title restoration and unrelated control sequences,
and complete/live tool output through actual registered renderers. Brand tests
first failed against upstream defaults. A stale input-hint redraw and missing
shell renderer were reproduced before correction, then passed.
Final visual QA retained the project name at the end of shortened directories
and fitted whole shortcut hints, with an additional failing-then-passing test.

The baseline rebuilt successfully for the existing start.cmd. A disposable
recipe with no terminal component also built, and its normal Cargo dependency
graph contained neither rig-tui nor ratatui. The no-default-features library
check, formatting and Git whitespace checks passed. Native linking emitted
MSVC's library/export-file informational warning; no new compiler warning was
introduced by the implementation.

The compact-screen script passed on real Windows ConPTY at 120x40 and 50x20,
both with the inherited NO_COLOR setting and with truecolor explicitly enabled:
branded startup/title, Plan-visible drafts, Unicode multiline continuation,
actual shell output, exit-7 failure cause, running output and Esc cancellation.
Synthetic terminal streams were rasterized and visually inspected at both sizes;
the images are ignored test artifacts, not screenshots of a real-provider task.
The scripted gateway checked that requests use the vivy-test default identity.
The existing Plan terminal script passed approval/write, feedback, scrolling,
modal exit and restart/resume. Subagent acceptance passed child views, usage,
parallel/chain workflows and cancellation of four shell descendants. The
existing general smoke passed model, file, shell, session, Python, JSON,
failure, credential-filtering, image, effort and compaction scenarios.

Test-only pyte 0.8.2, wcwidth 0.2.13 and pywinpty 3.1.0 were reused/installed in
ignored scratch and exposed through PYTHONPATH; no product dependency was added.
The new script closes its terminal before deleting its scratch directory and
uses process liveness on Windows, since pywinpty's output reader may wait after
the process exits. ConPTY multiline checks use the native backslash+Enter
fallback; the offline Paste test separately covers newlines and Unicode.

A fresh read-only source review found no substantiated issue. This review is
distinct from the builds and terminal runs above. All network acceptance here
used a synthetic loopback gateway; no new real-provider or Linux/macOS result
is claimed.
