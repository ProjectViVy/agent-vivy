# Agent Diva desktop integration plan

Status: **implementation started; incomplete**. Recorded on 2026-11-01.

The owner initially requested documentation only, then explicitly authorized
implementation. The progress record below distinguishes implemented work from
the remaining plan. No complete desktop integration or installer is claimed.

## 1. Goal and selected scope

Deliver an installable Windows desktop assistant using the existing Agent Diva
Tauri/Vue interface and the independent vivy-test Rust persistent-agent core.
Preserve YAML-selected compilation, one Rig execution path and existing durable
state ownership. The original Vivy checkout and its data remain out of scope.

| Decision | Selected behavior |
| --- | --- |
| Product face | Diva desktop face with a thin, recipe-selected Diva Front plugin |
| Process model | Tauri owns a bundled native core process over JSON Lines pipes |
| Window close | Hide to tray and continue running; explicit tray action exits |
| Data | Fresh Diva-owned home, workspace and persistent session |
| Workspace | One fixed workspace and one assistant; no project switching |
| Conversation | Text, streaming replies, tool status, questions, cancellation and recovery |
| Management | Providers, MCP, local Skills, persona/prompts, Recap, tasks and Dream |
| Background work | Automatic Recap and already scheduled work continue while hidden |
| Dream | Existing manual/tool-triggered workflow; no new periodic idle trigger |
| Voice | No transcription or synthesis; do not present local recordings as sent |
| Delivery | Windows x64 per-user NSIS installer; no Rust/Node.js required to run |

Retain the yellow/pink animated grain background, minimal chat, custom Windows
window controls, left hamburger and Zune-inspired settings layout. Do not restore
the removed extra chat header or settings breadcrumb. Coding Face and its recipe
remain separate. No coding-session delegation, online Skill marketplace, remote
service, Windows service, automatic updater or new plugin SDK is included.

## 2. Grounding and architecture choice

Source inspection established the following; these are not new execution results:

- The desktop repository is `.workspace/agent-diva-next`, a separate Git checkout.
  Its chat currently stores messages in Vue memory; its Rust command is `app_info`.
  Settings pages are placeholders.
- Rig's existing `PrintPlugin` JSON front supports `send`, `stop`, `answer` and
  basic `state`. Its private request enum is not a complete desktop management API.
- `vivy-durable::runtime::submit` already accepts source/request keys and exposes
  pending versus committed admission. Cancellation uses task generations.
- Provider configuration, credential storage, persona revisions, wake management,
  managed Skill publication and recovery already have backend implementations.
  Some management operations need small public typed entry points extracted from
  existing command handlers.
- Recap's automatic path excludes the `print` and `json` fronts. A `diva` front
  can use the existing automatic path; test this explicitly.
- AutoDream has manual/tool admission, validation and publication, but no current
  idle-periodic switch. Do not invent a UI switch for an absent scheduler.
- Persistent execution currently requires the local Rig checkpoint fork at
  `810d7dfd`; ordinary upstream pins alone do not provide this integration.

Approaches considered:

1. Extend the existing JSON front through management commands. Rejected for this
   scope: structured management responses, correlation and durable admission need
   more than its current private request set; parsing human notices is unsuitable.
2. Embed the core into Tauri. Deferred: it couples the generated composition root,
   application runner, build and shutdown lifecycle to the window process.
3. A thin Diva Front in a bundled core process. **Selected by the owner**: reuse
   core behavior and isolate desktop transport without modifying Rig references.

```text
Vue presentation and draft state
       <-> Tauri IPC
Tauri process owner and message bridge
       <-> UTF-8 JSON Lines over stdin/stdout
Diva Front
       <-> existing typed component APIs and Rig events
Vivy persistent assistant / Rig execution and storage
```

Backend changes belong to vivy-test; UI and desktop hosting belong to the separate
Agent Diva repository. Add `plugins/vivy-diva-front` and
`recipes/diva-agent.yaml`. The recipe selects persistent-assistant components and
the desktop front, omitting terminal-view components. Select the front using
`--diva`; preserve existing JSON/terminal behavior in their existing recipes.

Use Rig's headless runner, question handling, events and checkpoint machinery.
Do not add a second model loop, task database or scheduler. Keep domain operations
in their owning components; extract small typed functions shared by CLI handlers
and the desktop adapter. Capability availability comes from assembled components,
not a second hand-maintained catalog. Runtime settings configure compiled features.

## 3. Protocol and state contracts

### Transport and operations

- stdout contains protocol frames only; stderr contains bounded, redacted diagnostics.
- Requests carry protocol version and request ID. Events carry core-instance ID
  and an increasing sequence number. Handshake reports core/protocol versions,
  session identity, actual capabilities and initialization status.
- Reject incompatible protocols and unsupported operations with structured errors.
- Define protocol types in Rust; export TypeScript types and contract fixtures
  with the core artifact for consumption by the desktop repository.
- Use explicit typed operations rather than arbitrary shell commands, Python
  expressions, Rust method names or parsing CLI output.
- Long operations return an operation identity and publish progress/results.
  Distinguish transport receipt, running work and committed completion.

| Operation group | Required operations |
| --- | --- |
| Runtime | Handshake, snapshot, paginated history, shutdown |
| Conversation | Submit, admission lookup, task cancellation, answer questions |
| Providers | Read/save/remove, credential update, model selection, connection validation |
| MCP | Read/save/remove, enable/disable, connection check, reconnect/close, tool inventory |
| Skills | Inventory, local import, read/edit/remove/refresh, managed versions and rollback |
| Assistant | Persona read/update/history/rollback; prompt override read/save |
| Recap | Committed summary, coverage/pending/error status, refresh |
| Tasks | List/details/cancel, wake cancellation/rescheduling, uncertain-operation evidence |
| Dream | Request/status/cancel, validation jobs and detach |
| Environment | Python and optional memory readiness, initialize/retry/cancel |

### Submission and recovery

1. Assign a stable client request ID and persist its unconfirmed send in the
   Tauri-owned outbox before writing to the core pipe.
2. Show the existing user bubble as sending.
3. Submit through durable admission using source `diva` and the same request ID.
4. Acknowledge acceptance only after the matching admission is committed; return
   the task identity. Update the bubble and remove the outbox entry.

The outbox holds only unconfirmed submissions, not a second conversation/task
store. On restart, reconcile with the core before retrying. Reusing an ID with
the same content must not create another task; different content must fail.
Do not blindly replay management mutations with uncertain outcomes.

The core projects history from committed conversation/task records with stable
message identities. Distinguish user/assistant messages, tools and notices;
internal Dream requests and full model context are not ordinary chat history.
Streaming fragments are provisional. Only committed outcomes indicate completed
answers; interrupted provisional text must not become a fabricated final result.

Snapshot subscription must establish a consistent event watermark and apply only
later events. Sequence gaps or a changed core instance require resynchronization.
Paginate history and avoid retransmitting entire checkpoint logs for each update.
Question identities are scoped to the core instance; reject stale answers after
restart. Adapt the existing question/picker mechanisms rather than creating
another approval state machine.

### Files and process ownership

Under Tauri's resolved application-local-data directory, separate:

- Vivy home: settings, credentials and sessions;
- fixed workspace: persona, Skills, work files and evolution artifacts;
- desktop state: selected session, window state and unconfirmed sends;
- WebView2 data and diagnostic logs.

Create and save a valid session ID before normal use, then reopen that same
session. Reuse `SessionGuard` and reject competing writers. Corruption or schema
incompatibility must show its cause without replacing the session with an empty one.
Do not read, copy or interrupt existing trial sessions or their running processes.

Tauri starts only the bundled fixed-path core with explicit arguments, no visible
console and no shell-string construction. Own its process tree with a Windows
Job Object. Clear inherited launcher control context and explicitly set the
dedicated home/workspace/session. Preserve the existing credential filtering model.

Use a single desktop instance: subsequent launches reveal the existing window.
Window close hides it; tray actions open, open settings or exit. Explicit exit
stops new submissions, requests core shutdown and allows up to 10 seconds for
cleanup before terminating the owned process tree and recording abnormal exit.
Exit is not a blanket cancellation of all durable tasks; waiting tasks and unknown
effects retain existing recovery semantics. No wake is promised while the app is off.

Core crashes leave the window, draft and error visible with a reconnect action;
do not loop indefinitely restarting it. Host death must clean up owned descendants.
Create a writable, explicit WebView2 data directory so installed startup does not
depend on the temporary environment workaround used by previous UI previews.

## 4. Desktop behavior and management

### Chat and shared state

- Replace local-only send logic with the real bridge and one shared runtime/message
  state source. Components retain only drafts and local editing state.
- Distinguish user and assistant alignment. Show generating, tool execution,
  queued/waiting, question, stopped, failed and recovering states truthfully.
- Tool cards are collapsed by default with name, progress and outcome; expanded
  output is redacted. Render Markdown without raw HTML; code scrolls locally.
- Inline question cards support choices/text. Allow new text during generation,
  preserving the core's current input handoff semantics.
- The chat stop action targets the foreground task; other cancellations belong
  to the task page. Never cancel unrelated work just to refresh a screen.
- Preserve reading position when the user scrolls up and show a new-message cue.
- Voice remains visibly unavailable; do not record and append a local audio bubble
  that implies backend delivery. The undefined plus action remains outside scope.

### Settings structure

Retain the left category column, divider and right details; expand categories to
Providers, MCP, Skills, Persona and prompts, Recap, Tasks, Dream and Runtime.
Retain narrow-window list/detail navigation. Enter editing explicitly and save
explicitly. Failed saves preserve input and location. Only dirty-editor navigation
needs a discard/save decision. Scope busy states to the affected resource.

| Page | Required behavior |
| --- | --- |
| Providers | Manage instances for compiled protocols, endpoint, model facts, default model and reasoning; preserve model IDs exactly; reuse stored credentials/environment references; validate with a real minimal request and separately report text, streaming and tool results. |
| MCP | Manage existing stdio and Streamable HTTP fields, timeouts, tool filters and environment references; show connection/tool inventory; check, reconnect and close; no OAuth or new catalog. |
| Ordinary Skills | Import local directories into Diva-owned storage, inspect metadata/Markdown, edit SKILL.md, remove and refresh; reject name collisions, leave import sources untouched; removal moves the owned copy to an internal recovery directory. |
| Managed Skills | Show publications, candidates, validation, dependencies and rollback; edits create candidates through existing validation/publication, never mutate immutable published snapshots; no full code IDE. |
| Persona/prompts | Edit persona Markdown using revision/update-ID checks; inspect history and rollback; edit Assistant, Recap and AutoDream override files while preserving current precedence and retry freezing. |
| Recap | Show committed summary, coverage, pending work and failures; refresh; show optional memory readiness without adding a database editor. |
| Tasks | Inspect active/queued/waiting/suspended/completed tasks and operations; cancel individual tasks; cancel/reschedule existing wakes with their generations; expose uncertain effects and accept explicit reconciliation evidence. |
| Dream | Request with a reason, inspect/cancel, inspect candidates and validation jobs, detach waits and inspect publication results; no periodic reflection toggle. |
| Runtime | Core connection/version, data location, assembled capabilities/readiness, Python initialize/cancel/retry and redacted error/log access. |

Reuse provider settings conflict detection, persona revisions and publication CAS.
MCP changes apply after relevant active calls finish, with pending-application
status; do not reset the entire worker and interrupt unrelated work. Extend the
structured worker control channel for management, never evaluate generated Python
expressions for these operations.

Python setup is explicit and cancellable through the existing initialization
flow. Basic text chat works without Python. Missing Git Bash or other tool
dependencies produce accurate capability/readiness errors without blocking
unrelated functionality. Expose compiled/configured/ready as distinct states.

Do not return saved credentials to the UI or put them in frontend persistence,
chat history or logs. Reuse existing credential storage without claiming new
encryption guarantees. MCP settings continue to store environment-variable
references, not inline secret values. Local Skills and compiled plugins retain
the existing trust model; desktop management is not a sandbox.

### Background policy

Automatic Recap and existing wake handling continue while hidden. New foreground
input takes priority and uses the latest committed Recap without waiting for new
synthesis. Retain existing Dream manual/tool initiation and validation/publication.
Simply opening a management page must not start Dream, install dependencies or
perform connection tests.

## 5. Implementation sequence and packaging

M1 transport scaffolding is implemented; the full M1 contract and M2-M6 remain
incomplete. See the progress record for exact executed checks.

| Milestone | Work | Exit condition |
| --- | --- | --- |
| M1: Desktop core | Recipe, Front, protocol, handshake, capabilities, session entry | Native pipe startup/query/shutdown works; existing fronts remain compatible |
| M2: Real conversation | Durable admission, streaming/tools/questions, cancellation, snapshot/history | A real task runs, stops and restores after process restart |
| M3: Desktop host | Tauri bridge, single instance, tray, process ownership, data/WebView2 paths | Window closes to continued execution; explicit exit cleans up processes |
| M4: Management | All management pages and underlying typed operations | Visible operations persist real results and survive reopening |
| M5: Failure/UI acceptance | Crashes, writes, conflicts, disconnects, narrow layouts and long content | Failures remain explainable/recoverable without duplicate execution |
| M6: Installer | Artifact assembly, installation, upgrade, clean-user testing | Basic chat runs without source checkout, Rust or Node.js |

Build a Windows x64 per-user NSIS installer carrying a matching core. Use Tauri's
external-binary packaging and system tray support:
[sidecars](https://v2.tauri.app/develop/sidecar/) and
[system tray](https://v2.tauri.app/learn/system-tray/).

- Record the full checkpoint-fork commit, recipe and lock versions in build
  metadata. Resolve the inspected short baseline `810d7dfd` to its full commit.
  Explicitly supply the Rig source path; do not modify reference checkouts or
  silently substitute upstream. This is build pinning, not a provenance platform.
- Export the core binary, protocol types/version and required licenses as a
  matching artifact. The desktop packager rejects missing/incompatible artifacts.
- Do not add developer-absolute Cargo paths to the desktop repository or copy the
  entire backend source tree into it. Local integration scripts build the core
  first and assemble the desktop second.
- Preserve frontend/bridge CI checks. Full packaging requires the matching core
  artifact as an explicit input; absence is an error, not a successful partial build.
- Include WebView2 setup handling. Python and MCP server dependencies remain
  explicit optional setup rather than prerequisites for basic text chat.
- Use short build paths and writable temporary directories, and test a clean build
  against the recorded MSVC temporary-file issue rather than relying only on caches.
- Upgrades preserve data. Unsupported saved schemas refuse to open without
  rewriting. Uninstall preserves user data by default.
- Deliver the installer locally first. Signing, automatic updates and public
  release are not prerequisites. Commit focused related paths in each repository;
  publication/push must follow the applicable user authorization.

## 6. Verification and acceptance

### Core and protocol

- Recipe parse/error checks and selected-dependency evidence: desktop Front is
  included, terminal view omitted; existing recipes still work.
- JSON framing, Unicode/multiline text, invalid frames, unknown methods and
  incompatible versions. Bounded transport failures retain useful causes.
- Duplicate identical submissions create one task; conflicting reuse fails.
  Kill before/after committed admission and reconcile the outbox on reopen.
- Exercise text, streaming, tools, questions, cancellation, waiting/wake and silent
  completion through the actual native executable with a synthetic gateway.
- Snapshot/event boundaries and history pagination do not duplicate or omit
  committed messages. Provisional text is not reported as durable completion.
- Do not replay unknown tool effects; stale cancellation generations cannot
  affect later work. Competing writers, corrupted logs, append failures and
  schema mismatches preserve causes and original data.
- Run Rust formatting checks, focused offline tests, builds and native smoke
  appropriate to changed modules. Source inspection is not execution evidence.

### Management

- Providers: save/remove/key updates, external-file conflicts, invalid endpoints,
  authentication failures and preserved model identifiers.
- MCP: actual stdio/HTTP configuration, connection/tool inventory, timeouts,
  pending apply and cancellation without interrupting unrelated work.
- Skills: collisions, invalid files, path escape, edit conflicts, recovery-directory
  removal and refreshed inventory. Failed managed validation preserves current;
  successful publication and rollback retain frozen call/version semantics.
- Persona: concurrent revision edits and rollback. Prompt changes do not alter
  already frozen retries. Recap covers failure/refresh and foreground preemption.
- Dream: request/cancel, candidate validation, publication and detach behavior.
- Python: failed/cancelled setup and retry preserve the previous usable environment.
- Verify credentials are absent from snapshots, frontend persistence, logs and
  chat output using dedicated test values, not private production data.

### UI and Windows installation

- Focused frontend tests cover event merging, synchronization, editing and failed
  saves. Browser tests use an explicitly mocked IPC adapter.
- Real desktop acceptance must use Tauri, WebView2 and the native core; browser
  tests or a successful build cannot establish desktop behavior.
- Check 500x400, the 680px breakpoint, 1100x760, maximized windows and Windows
  125%/150% scaling. Include long messages/code/tool output/paths, form errors and
  concurrent resource operations; keep input/actions reachable.
- Hide to tray during a real wait/wake task; reopen and see the resulting state.
  Check cleanup after explicit exit and forced Tauri termination, including core,
  Python and MCP descendants.
- Install as an ordinary user without source/Rust/Node.js, configure a test gateway,
  converse, exit/reopen and upgrade while retaining data. Test the explicit
  WebView2 data path instead of a development-only launch workaround.
- Use synthetic gateways for deterministic recovery/failure tests. Separately
  validate streaming and tool calls on the actual intended gateway; if credentials
  are unavailable, record this as unverified rather than claiming compatibility.

Final acceptance requires installation, real chat, complete local management,
continued execution while hidden and correct recovery after exit/reopen. Update
the existing DESIGN/HANDOFF records with actual implementation, build inputs,
measured checks and remaining limits when implementation eventually occurs.

## 7. Implementation progress (2026-11-01)

Worktree: `.workspace/diva-integration`, branch `feat/diva-desktop`.

- Added the desktop recipe and `DivaFrontPlugin`, using the existing headless
  runner, a bounded input channel, 1 MiB input limit, versioned output, instance
  identity, sequence numbers, runtime-derived plugin inventory, snapshot and
  shutdown. No UI or reference checkout changes.
- Native smoke initially failed because the binary did not exist. After the
  first build it exposed an actual first-run exit: the shared headless driver
  exits without a reachable model. Added the existing `Starting` barrier while
  awaiting model configuration so management stays reachable. This does not add
  a model loop or modify Rig.
- Native `scripts/diva_front_smoke.py C:/vtd0/rig-dev/debug/diva-agent.exe` passed:
  fresh temporary home, ready/session/inventory, version and unknown-operation
  rejection, Unicode request correlation, snapshot and orderly exit.
- `cargo test -p vivy --offline` passed. Generated composition loads the desktop
  plugin and omits the terminal plugin. The generated patch table mentions
  rig-tui because it lists local Rig packages; that is not compilation evidence.
- Build uses `CARGO_TARGET_DIR=C:/vtd0`, `CARGO_PROFILE_DEV_DEBUG=0`,
  `TEMP=TMP=C:/vtd0/tmp` and the existing `.workspace/rig-durable` checkout.

Next: expand protocol tests for EOF/oversize/reopen and type export, then implement
committed admission, question adaptation, streaming and historical projections
before wiring the desktop UI. No real gateway, complete M1 capability/readiness
contract, M2 conversation, management pages, tray host or installer is verified.

### Native transport boundary follow-up (2026-11-01)

- Added distinct `incompatible_version` and `unsupported_operation` error codes.
  Invalid/oversized correlation IDs are not reflected in error responses. Input
  pipe read failures retain their OS cause without echoing request contents.
- JSON Lines frames now require their newline terminator. A native regression
  demonstrated that the previous implementation executed a valid JSON object
  after EOF without its terminator; it now reports `invalid_frame` and exits.
  Oversized frames likewise report an error and close the connection.
- Added `scripts/diva_transport_test.py`, using disposable homes and actual
  executables, covering EOF exit and session reopen, invalid UTF-8/JSON and unknown
  fields, Unicode/multiline request IDs, oversize input, and incomplete frames.
- Executed: four native boundary tests and the existing native front smoke passed;
  `cargo test --workspace --locked --offline` passed all 34 tests; plugin formatting
  and Git whitespace checks passed. Rebuilt the native recipe with the same short
  target/temp paths and explicit Rig fork. Existing unused-patch, unused-provider
  function and MSVC linker warnings remain.
- This is a transport increment, not completion of M1 or desktop delivery. Type
  export and capability/readiness contracts remain next, followed by M2-M6.

### Continued implementation (2026-11-02)

The owner authorized continuous implementation of M1–M6 in the follow-up plan.
Backend base: `fe9e98b` on `feat/diva-desktop`; desktop base: `4633adbb` on
`feat/diva-core-integration`. No merge or push is authorized.

Interface audit: the desktop consumes the exported Rust protocol and matching
binary; admission acknowledgements must come from committed core state before
removing desktop outbox records. Snapshot watermarks and stream events share one
instance/sequence. Management adapters retain each component's storage owner.

- M1: Rust DTOs now export TypeScript, protocol version and serialization examples.
  Snapshot includes the runtime-selected foreground assistant, compiled plugin
  inventory and separate model/Python/memory readiness. Initial Python detection
  can be `checking`; it is not a prepared or ready environment.
- Pipe writes use a bounded background queue. Saturation/disconnection terminates
  the transport; normal shutdown/EOF drains queued replies before exiting.
- Verification: 34 root tests passed; two Rust contract tests passed; five native
  boundary tests and the native front smoke passed. The new handshake assertion
  first failed against the base binary because the watermark was absent.
- Remaining: M2 durable chat and recovery; M3 native host; all M4 management;
  M5 failure/UI acceptance; M6 artifact, installer and actual-provider acceptance.
  No completed desktop or installer is claimed.

### Conversation and provider adapter increment (2026-11-02)

- Added source=`diva` admission/reconciliation, pending-versus-committed replies,
  receipt-gated admission/cancellation events, generation-fenced stop, instance-bound
  questions, provisional stream projection and committed history pagination.
  Message IDs come from saved task/operation identities. Model request/context
  bodies and Dream internals are not returned as ordinary history.
- Provider configuration and credential writes reuse the existing settings CAS
  and credential owner. Explicit default selection reconnects at the idle boundary.
- Four native scenarios pass: Unicode admission/dedup/conflict/history/reopen;
  slow-model generation cancellation and stale answer; provider CAS/external edits/
  write-only credentials; process death followed by admission reconciliation.
  The transport smoke and five boundary tests also pass, as do the two contract
  tests. Broader tool/fault/UI/installer acceptance remains outstanding.
- The supplied desktop gateway was attempted with fresh disposable data; the first
  Rig run failed before producing stream/tool evidence. Diagnosis remains open;
  this is not a successful real-provider acceptance.

### Management, desktop host and verification continuation (2026-11-02)

- Domain adapters now cover Providers (including separate fixed text/stream/tool
  probes), MCP, ordinary and managed Skills, Persona/prompts, Recap, Tasks/Dream
  and Runtime/Python preparation. Reads do not initiate preparation, connections
  or Dream. Long work returns an operation receipt and a separate result event.
- MCP stdio/HTTP fields and environment references are typed. Unknown configuration
  fields and literal credentials are rejected; absent tool filters differ from an
  explicit empty allowlist. Worker controls use JSON messages, never generated
  Python expressions. Reload waits on the affected server's existing call lock;
  other workers compare current configuration before their next call.
- Ordinary skill imports copy bounded validated directories into internal storage,
  reject reparse points/path escape/name collisions, use revision checks and move
  removals into recovery. Managed edits freeze new candidates; validation receipts
  live only in the current process. Publish and rollback retain existing CAS.
- Dream turn cancellation, validation-job cancellation and detach are separate
  controls. Wake and effect evidence controls retain generation/revision checks;
  completed UI controls wait for persisted receipts.
- Tauri's fixed bundled child starts suspended, enters a kill-on-close Windows
  Job Object and then resumes. The host persists its session before launch and its
  outbox before writing stdin; it queries admissions after reconnect. Close hides,
  tray Exit drains shutdown for up to ten seconds, single-instance activation shows
  the existing window, and a crash requires explicit reconnect. Logs rotate and
  retain redacted startup causes. No arbitrary process command is exposed to IPC.
- Vue uses one conversation runtime, generated protocol DTOs, receipt-aware control
  waits, interrupted provisional text, question/tool cards and disabled raw HTML.
  History cursor expiry restarts projection reads; an in-flight snapshot cannot
  resurrect a disconnected connection. Editors retain failed input and prompt only
  when leaving dirty edits. Explicit test-mode IPC is absent from production output.
- Executed: nine `diva_conversation_test.py` cases, five `diva_transport_test.py`
  cases and `diva_front_smoke.py`; Python Rust 39, durable/autodream 47 and Front
  contract 2 tests. Earlier affected basics 64 and root 34 tests passed. Native
  Python MCP 7 and runtime 8 cases also passed (including real local stdio and
  Streamable HTTP). No private sessions were read.
- Desktop `cargo test --manifest-path bridge-tests/Cargo.toml --offline --
  --include-ignored` with explicit `DIVA_TEST_CORE` passed 10 cases plus a fixture.
  This includes write-failure no-send, committed admission dedup after a killed
  core, session reuse, preserved malformed settings and concurrent session writer
  rejection. Both normal drop and forced host death terminate Python descendants.
  `npm test` passes 6 runtime cases; `npm run build` passes.
- Browser mock checks: failed send retains draft, provider conflict retains editor
  input, dirty leave offers Save/Discard/Stay, all eight pages render. Actual CSS
  500x400, 680/681px breakpoint and 1100x760 have no page horizontal overflow.
  This is Chrome synthetic IPC evidence, not Tauri/WebView2 or OS-scale acceptance.
- The supplied StepFun endpoint/model failed native HTTP transport after five
  retries, both inherited-proxy and no-proxy runs. The supplied Qwen endpoint's
  exact model returned HTTP 404 `model_not_found`. Actual gateway acceptance is
  still blocked. Native app automation reports an unavailable local control pipe;
  no unrelated remote desktop was used. Standard-user install, WebView2 GUI,
  125/150% native scale, hidden Recap/wake and upgrade acceptance remain pending.
- Core exporter records backend/Rig revisions, protocol, recipe/lock hashes and
  dependency licenses. Desktop import verifies every artifact file, and build
  fails on absent/mismatched binary, Rust-generated types or dependency notices.
  CI separates frontend/bridge checks from explicit-artifact NSIS packaging.
  Installer generation and independent whole-implementation review are underway;
  these statements do not mark M1–M6 or final product acceptance complete.

### Independent review fixes (2026-11-02)

The read-only whole-implementation review found four substantive defects. Each
was reproduced or covered by a focused regression before packaging:

- MCP forms now distinguish all tools from an explicit empty allowlist. A mounted
  Vue editor regression first failed on an unrelated timeout edit and now passes.
- History pagination budgets serialized JSON bytes, including escaping. Very long
  message/tool previews carry a truncation flag; originals remain in committed
  records. Ten native conversation cases now pass, including nine 500 KiB user
  admissions, cancellation and multi-page history with stable unique IDs.
- Managed desktop import enforces its editor's 512 KiB UTF-8 contract before
  candidate admission. Oversized/non-text input cannot poison the candidate list.
- MCP save registers credential references with the process owner. Python workers
  inherit an explicit OS-runtime environment, not arbitrary host variables. The
  host continues resolving MCP references. Newly registered ordinary references
  do not reset the worker or discard variables; reserved runtime-variable names
  cannot be used as MCP credential references. A subprocess regression verifies
  the secret is absent before and after save and the Python marker survives.

Post-fix checks: Python Rust 40, Front contract/projection 3, root 34, native
conversation 10, Python MCP 7, Python runtime 8, desktop bridge 10 plus one fixture,
and Vue 7 pass. Production Vue compilation passes. Remaining external acceptance
limitations above are unchanged; these are fixes, not a full release sign-off.

Two additional native cases pass (12 total): the model requests the actual Rig
read tool against a disposable file and its committed result appears in history;
an actual project-trust question accepts a current answer and rejects invalid or
already-settled answers. Core distribution currently uses the tested dev-profile
executable; metadata records the profile/compiler/lockfile version explicitly.
Desktop packaging uses Tauri's release profile. No optimization claim is made.
