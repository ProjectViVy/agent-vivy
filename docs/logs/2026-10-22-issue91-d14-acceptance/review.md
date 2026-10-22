# Independent behavior and residue reviews

Final executable candidate: `5b0b572da80602f46d7496d004aa9d88fc6f5eeb`.
Both reviewers are independent of the implementation owners. Their scoped
source verdicts complement the platform and real-path gates in verification.md;
they do not supply the owner's pending real-model verdict.

## Findings and corrections

At a12748a7, neither reviewer accepted. The behavior reviewer found paid warming
outside BudgetLedger admission/settlement and WaitIdle drainage. The existing
owner now admits and settles request/usage/finish, propagates persistence
failures, and drains before terminal/idle and same-Run steering handoff. Native
Eino generation and the single Service/Journal/policy path remain authoritative.

The residue reviewer traced active custom identities in steering, native
orchestration, ActionHost audit, operation/channel/observer/memory/MCP/workspace
identities, provider cache and TUI cache. DTO/schema/generator/i18n consumers and
current instructions were corrected with their producers. RPC now recognizes
the current redacted bash preview. These were owner fixes, followed by renewed
review, rather than directory-wide exemptions.

Native acceptance exposed a proven-pristine BML file left by cancelled schema
initialization. Its reader now recognizes that exact empty shape without
initializing it; unknown/schema1 files still refuse unchanged. The approval
conformance test now verifies durable outcome before actual provider entry,
exact call/name/result and one terminal, while allowing display projections to
lag under design 11.4. Persona's real browser cold load exposed a session-binding
race; the existing messagesPhase gates actions, with existing host retry on
failure. Its authorization and stale-response checks remain intact.

The masks browser helper now waits for changed session identity, ready messages
and completion of existing session creation before navigating. The governance
approval helper now waits for the matching durable Journal event, not just an
earlier SQL row. These verification corrections retain the original assertions,
timeouts and product behavior; all redundant not-durable retries were removed.

At 9214f200, the behavior reviewer independently reproduced a distinct P1 in
both root approval and question settlement. Publication preceded private
pending registration; a fast decision/answer could settle successfully and
leave the Run stranded. Prior conditional behavior acceptance was withdrawn.
An actual owner-ordering fixture produced durable=true, before_pending=false,
successful settlement, after_pending=true and no terminal on both paths.
This first fixture used opaque checkpoint bytes and did not claim Eino
continuation. The implementation owner's separate regression uses actual
Service.drive and native Eino checkpoints and verifies continued execution.
The first correction at86215cf2 fixed root chat/question paths and deadlines,
but wrongly excluded native workflow approvals. Fresh independent execution
with a real Eino workflow/checkpoint reproduced that route in0.113s: durable
request, free projection lock, early successful decision and no pending resume.
That candidate was not accepted. The final correction includes native workflow
approvals in the same readiness rule. The workflow caller consumes the direct
approval-registration outcome rather than re-reading a map entry a fast decider
may already have consumed. This latter ambiguity was source-supported, not a
separately reproduced P1. No new runtime state or publication barrier is added.

## Final verdict and retained boundaries

Both independent reviewers accept exact `5b0b572da80602f46d7496d004aa9d88fc6f5eeb`.
Their final delta review is source-only. From `73ef094c` to `5b0b572d`, five obsolete assertions
were removed from the independent session-tree, evolution, memory, notebook and
persona Module tests, and two UI locale descriptions were corrected. Go/runtime
behavior, schemas, manifests and pins are unchanged from `73ef094c`; the locale changes
only clarify when saved credentials apply. Module/Port/Provider/Ref/catalog/
lifecycle assertions remain intact. No new runtime test execution is attributed
to this delta review.

The native73 full gate exposed those five active Source.SHA256 consumers after
all root Go/SDK and headless checks passed. The residue reviewer's earlier73
verdict missed them and was incomplete. They were compile/retirement blockers,
not historical fixtures. At09 the full tracked executable/test/config scan has
no active reads of the removed field class. The broader source/asset/lock field
sweep retains only negative assertions at sdk/internal/assembly/
d06_identity_test.go:74 and ui_test.go:304 that prevent retired fields from
regenerating. sdk/module.Source remains Ref-only. This closes the missed class;
no independent Module or test directory receives a blanket exemption.

Fresh independent execution remains attributed to73:30 tests/subtests pass,
zero failures/skips, comprising an actual-workflow overlay1/0.276s, runtime
owners26/18.184s and App nonce/held-Journal/redaction3/0.451s. Both commands
exited0 in a clean exact-SHA Linux checkout with GOMAXPROCS=2, GOFLAGS=-p=1 and
count1. The independent RED862 -> GREEN73 proves a waiting response preserves
its durable slot, then native Eino resumes node A once, preserves completed
sibling B and emits one terminal. Targeted cases also cover root approval,
question, timeout/retry, cancellation, expiry, child, shell, restart recovery
and system auto-approval. That Go/runtime behavior is unchanged at `5b0b572d`. No extra full suite
or race execution is attributed to this reviewer.

Earlier independent execution at1b8da8cc passed143 tests/subtests with zero
skips (runtime11.001s, App0.201s); the f6 delta passed seven more, including
pristine BML and causal approval. They remain named historical executions.

The residue review applied SQL001..047:71 SQLite objects, with no final
hash/digest/checksum schema field. Remaining occurrences have specific purposes:
native Git/dependency/delivery integrity, immutable historical SQL followed by
final DDL, explicit old-shape refusal, inert refusal/negative test literals,
URL fragments and ordinary summaries. PostgreSQL hashtextextended is the native
advisory-lock key primitive, not persisted content authority. Those production,
schema, generator and dependency sources remain identical from73 to09.

Exact independent RED/GREEN sources and raw results are preserved in the stable
checkout's ignored .workspace/issue91-final/independent-review-evidence/.
Temporary audit overlays and its verified TEMP task folder were removed after
the evidence copy; committed owner regressions provide durable reproduction.

INOFY's completed-resume changed-input matrix remains unclaimed: the current
production caller does not expose Resume and its RPC is explicitly unsupported.
Evolution remains the deferred UI-EVO demo. Later #93 extraction/physical
omission, absent later-baseline features, and the full external Studio shell
are outside this delivery. Finite Studio ledger conversion has scoped evidence.
The owner's real-model tiny-Face verdict remains last and pending. Neither
reviewer substitutes source acceptance for the integrated platform, artifact
or human gates recorded separately.
