# Toolkit shared contracts

**Revision:** TK-C1, 2026-10-11. Proposed implementation contracts.\
**Architecture:** [Vivy Toolkits](../../../architecture/VIVY-CONTENT-BUNDLES.md), r2 plus the planning refinements in section 22.\
**Status and predecessors:** [index](index.md). This file owns cross-Story signatures; Story plans must reference these names rather than invent alternatives.

## C1 Names and boundaries

Product copy is **Toolkit / Toolkits / 工具包**. The content encoding remains `vivy-bundle.json`, `vivy.bundle/v1`, and package namespace `internal/bundlecontract` / `internal/bundles`. Existing compiled Modules and upstream projects retain their Plugin terminology. No upstream importer, converter, or compatibility mode is included.

Backend control owner: proposed compiled T1 `vivy/toolkits` under `internal/modules/toolkits`, providing only the existing `std/control-action@v1`. It is a thin action inventory; the manager stays in `internal/bundles` and authority stays in ActionHost/app/Core Storage. UI: proposed compiled `vivy/toolkits-ui`, `repo:plugins/vivy-toolkits-ui`, providing existing `std/ui-extension@v1`. A toolkit artifact provides neither Module. Default Recipe selects both compiled surfaces; omission fixtures remove the UI and action owner independently. Do not add a public bundle Port or a closed factory Port just to transport an app-owned manager.

The app constructs the manager only after existing storage and organism-lease readiness. It injects a narrow Manager into a private ActionHost facade restricted to the exact compiler-bound T1 owner. Mirror the established mask action binding pattern; do not grant raw storage to public actions. Source hashes and generated wiring come from the existing SDK pipeline.

## C2 Native values and validation

Proposed files: `internal/bundlecontract/{manifest,identity,catalog,operations,errors}.go`. Types contain no Eino, SQL, RPC or process dependencies. Existing domain RunID/SessionID types remain authoritative.

| Value | Exact fields or rule |
| --- | --- |
| `ComponentKind` | String enum `skill`, `mask`, `rule`, `hook`, `mcp` |
| `ComponentRef` | `InstallID string; Kind ComponentKind; LocalID string` |
| `ReleaseRef` | `InstallID, BundleID, ReleaseDigest string; ReleaseSequence int64` |
| `CatalogIdentity` | `Epoch, ConfigRevision int64; EffectiveDigest, GenerationID string` |
| `CaptureCheck` | `Catalog CatalogIdentity; RevocationRevision int64` |
| `SourceSpec` | `Kind string` (`local` or `git`), `Location string`, `Ref string`; Ref only for Git |
| `SourceLock` | Kind, redacted location/origin, requested ref, resolved commit or local acquisition digest, validator version; no credential values |
| `InventoryEntry` | `Path string; Size int64; SHA256 string` |
| `Manifest` | Fields exactly matching architecture section 5; `Components` has typed slices for all five kinds |
| `SkillEntry` | `ID, Path string` |
| `MaskEntry` | `ID, Name, Description, Path string; RecommendedSkills []string` |
| `RuleEntry` | `ID, Path string; Faces []string` (empty means all supported Faces) |
| `HookEntry` | `ID, Event, Command, Matcher, Cwd string; TimeoutMs int; EnvFrom map[string]string` |
| `MCPEntry` | `ID, Endpoint, Command, Cwd, AuthEnv string; Args []string; EnvFrom map[string]string; ResourceBridge bool` |

Use JSON tags from the native manifest (`apiVersion`, `recommendedSkills`, `timeoutMs`, `envFrom`, `authEnv`, `resourceBridge`). Control-action request/response fields use existing snake_case convention. Keep these two encodings separate and test their mapping.

Copy all native identifier, path, byte and count bounds from architecture sections 5–7. Unknown/duplicate keys and unsupported schemas fail. No `approved`, `enabled`, executable environment literals, Model/Agent overrides or policy fields in native declarations. Default arrays are empty, resourceBridge=false, process cwd=workspace. Event is exactly `PreToolUse`. Timeout must be positive and within the configured Host bound.

Native SKILL.md supports YAML frontmatter `name` and required `description`, followed by literal body. A missing name uses the qualified display reference. No other frontmatter key is accepted for toolkit Skills. Project/user Skill parsing retains its current wider contract. Reuse the existing YAML dependency with KnownFields and explicit duplicate-node detection; do not import Runtime or Eino into the bundle manager. Toolkit Skills are on-demand, user-invocable, inline, and declare no automatic Tool mounts.

Proposed functions implemented by TK-01:

```go
func DecodeManifest(raw []byte) (Manifest, error)
func ValidateManifest(m Manifest, limits Limits) error
func DecodeNativeSkill(raw []byte) (NativeSkill, error)
func ValidateRelativePath(path string) error
func SkillID(installID, localID string) (string, error)
func MCPInstanceID(installID, localID string) (string, error)
func MaskID(installID, localID string) (string, error)
func CanonicalManifest(m Manifest) ([]byte, error)
func ArtifactDigest(m Manifest, inventory []InventoryEntry) (string, error)
```

`NativeSkill` contains `Name, Description, Body string`. `Limits` contains the named size/count bounds in architecture section 5.2 plus `HookTimeoutMax time.Duration`; defaults are defined once in this package. ID functions return `invalid_manifest` on invalid inputs, never sanitize. Digest recipe is architecture section 7.2, including domain separation and length prefixes. The original manifest must be present during acquisition; stored canonical-manifest plus declared raw files reproduce the digest. Inventory is a set keyed by path even if several components reference the same file.

## C3 Artifacts and projections

TK-02 implements in `internal/bundles`:

```go
func Acquire(ctx context.Context, in AcquireRequest) (Artifact, error)
func OpenArtifact(ctx context.Context, root, digest string) (Artifact, error)
func ValidateArtifact(ctx context.Context, a Artifact, caps Capabilities) error
func (a Artifact) ReadDeclared(ctx context.Context, path string, maxBytes int64) ([]byte, error)
```

`AcquireRequest` contains OperationID, Source SourceSpec, StateRoot, and Limits. `Artifact` exposes Manifest, ReleaseDigest, Inventory and SourceLock by copy; its managed directory/handles remain private. `Capabilities` is a compiled-owner support set plus current owner limits, derived by app, never supplied by the artifact. Reads verify confinement and hash. Do not return a mutable map or open filesystem handle to a public Provider.

Acquisition is cancellable and process-bounded. Git command arguments are separate argv values; disable repo hooks, filters, LFS and submodule execution. Source credentials are supplied by the existing host credential mechanism, not logged or persisted in URLs. A local stage copies and validates regular files; reject links/special files/case collisions. Promote objects on the same filesystem before committing references. Objects are immutable. On cancellation remove only the operation's unreferenced staging directory.

`View` is an immutable bundlecontract value with Catalog identity, sorted ReleaseRefs, normalized component records, configured executable references and safe diagnostics. Component records carry ComponentRef, stable projected ID, ReleaseDigest, owner content digest and source display metadata. View contains no live client, process or secret. Constructor validates uniqueness; accessor methods return copies. The app may hold private prepared owner handles beside it, never serialize them.

## C4 Storage and transactional authority

TK-03 adds `internal/storage/bundles.go`; SQL stays in paired migrations/backends. At inspected baseline migrations end at 040. Reserve `041_content_bundles.sql` for this lane only after checking both dialects at execution; if another lane has taken it, select the next common unused number and update the index before migration work. Never alter a released migration.

Logical tables: `bundle_installations`, `bundle_releases`, `bundle_catalog_epochs`, singleton `bundle_catalog_state`, `bundle_operations`, `bundle_consents`, `bundle_revocations`, `run_bundle_snapshots`, `bundle_artifact_refs`. Store artifact-derived component bodies only in immutable files, not a second editable SQL catalog. Snapshot payloads are execution evidence. Backends share logical fields, unique live BundleID semantics, CAS and receipt behavior.

The initial empty epoch is 0. Epoch increments on successful publication only; installation metadata and operation receipts can change without publishing an enabled catalog. ReleaseSequence increases per installation including rollback; the same ReleaseDigest may recur at a later sequence. OperationID is UUID; identical input hashes replay the committed receipt, different hashes return `revision_conflict`. InstallID tombstones are never reused.

Proposed Store contract implemented by both backends:

```go
type BundleStore interface {
    CheckBundleLease(context.Context, string) error
    ReadBundleState(context.Context) (BundleState, error)
    SaveBundleInstallation(context.Context, InstallCommit) (Operation, error)
    BeginBundleOperation(context.Context, BeginOperation) (Operation, error)
    GetBundleOperation(context.Context, string) (Operation, error)
    CancelBundleOperation(context.Context, CancelOperation) (Operation, error)
    ListBundleBlockers(context.Context, BlockerQuery) (BlockerPage, error)
    CommitBundleActivation(context.Context, ActivationCommit) (Operation, error)
    CommitBundleConsent(context.Context, ConsentCommit) (Operation, error)
    CommitBundleRevocation(context.Context, RevocationCommit) (Operation, error)
    SaveRunBundleSnapshot(context.Context, RunBundleSnapshot) error
    LoadRunBundleSnapshot(context.Context, domain.RunID) (RunBundleSnapshot, error)
    ReadBundleReachability(context.Context) (ArtifactReachability, error)
}
```

`SaveBundleInstallation` rechecks the expected full CatalogIdentity inside the same transaction that writes the disabled install and receipt. The explicit `CheckBundleLease` read is reserved for reference-derived GC; activation and consent still validate the lease in their mutation transaction.

Mutation records contain OperationID, InputDigest, expected CatalogIdentity/config/consent revisions as applicable, authenticated audit subject, and typed payload. `ActivationCommit` additionally contains candidate View identity/release references, desired install states and operation result. It checks no nonterminal runs, Mask references, lease validity, consent and catalog CAS in one transaction. Consent commits grant only exact current/candidate declaration hashes; revocations monotonically increment the fence revision. `CancelOperation` includes the expected operation revision and cannot cancel a committed publication or erase a fence.

`BundleState` contains installations, releases, current CatalogIdentity, effective binding references, pending operation and consent/fence revisions. Paged queries have bounded limit (default 50, maximum 100) and opaque cursor; row IDs are not implicit authority. `Operation` fields: ID, Kind, State, Revision, ExpectedEpoch, CommittedEpoch, safe ErrorCode, Retryable, Progress summary and bounded blockers. `ArtifactReachability` returns digest references from install/current/pending/run/checkpoint evidence. Handle references are joined by the manager before GC.

`RunBundleSnapshot` contains RunID, SchemaVersion=1, immutable Payload bytes and PayloadSHA256. Payload fields follow architecture section 14.2. Extend ordinary, continuity, goal, primary/queue and child admission values with `Bundle *RunBundleSnapshot` and `ExpectedBundle *CaptureCheck`. Validate/insert them within each owner's existing transaction alongside Prompt/ExpectedMask. Do not add a post-commit repair writer. Old runs without bundle markers remain valid; a new started marker requires its row.

## C5 Coordinator and Runtime integration

TK-04 manager/coordinator uses the following seams (implement in manager/app; no Runtime import into the manager):

```go
type AdmissionGate interface {
    BeginDrain(ctx context.Context, operationID string) error
    WaitIdle(ctx context.Context, operationID string) error
    EndDrain(operationID string)
}
type PreparedOwners interface {
    View() bundlecontract.View
    Publish()                 // infallible pointer exposure after durable commit
    Close(context.Context) error
}
type PrepareOwners func(context.Context, bundlecontract.View) (PreparedOwners, error)
func NewManager(cfg Config, store storage.BundleStore) (*Manager, error)
func (m *Manager) Preview(ctx context.Context, req PreviewRequest) (PreparedSource, error)
func (m *Manager) Install(ctx context.Context, req InstallRequest) (Operation, error)
func (m *Manager) Recover(ctx context.Context) error
func (m *Manager) Prepare(ctx context.Context, req PrepareRequest) (PreparedCatalog, error)
func (m *Manager) Publish(ctx context.Context, operationID string) (Operation, error)
func (m *Manager) AcquireView(ctx context.Context) (ViewHandle, error)
func (m *Manager) CheckLive(ctx context.Context, check CaptureCheck) error
func (m *Manager) Close(ctx context.Context) error
```

`Config` supplies StateRoot, Limits, Capabilities, Gate, PrepareOwners and the existing lease/credential owners via narrow callbacks. `ViewHandle` returns an immutable View and an idempotent Release. `PrepareRequest` contains operation identity, Expected CatalogIdentity and `CatalogMutation` (kind plus one typed mutation payload). `PreparedCatalog` carries the expected identity, candidate digest and release references as in architecture section 4.2. It is private staging evidence; only the coordinator publishes.

`PreviewRequest` contains OperationID, Expected CatalogIdentity and SourceSpec. `PreparedSource` contains preview OperationID, a server-reserved InstallID, acquired ReleaseDigest, safe SourceLock, validated inventory summary and diagnostics. The reserved ID makes previewed component identities stable; install consumes that ID rather than generating a different projection. It references a managed staged immutable artifact with no installation row or activation. `InstallRequest` contains its own OperationID, Expected CatalogIdentity, PreviewOperationID and ExpectedReleaseDigest. Install revalidates that exact retained candidate, rejects cancelled/missing/stale previews, and commits disabled installation once. It never refetches a mutable source after the user's preview. Preview cancellation releases its object reference through normal GC. Preview consent is not executable consent.

Publication order: prepare static objects; persist pending operation; BeginDrain; WaitIdle; recheck all CAS/reference predicates; commit activation; Publish one pointer; EndDrain. On precommit failure close candidate and keep old view. Postcommit failure keeps admission closed and enters recovery; never silently fall back. No database transaction spans drain wait or network calls. Unrelated settings keep existing paths; shared MCP/engine content configuration flows through this coordinator.

TK-05 implements AdmissionGate on existing Service. Root admissions during draining return `catalog_update_pending`; queued messages remain queued. Resume and child runs from admitted parents use their captured epoch and may finish. WaitIdle combines durable nonterminal runs, active iterators, admission reservations, and in-flight tool calls; session count alone is irrelevant. Resume loads exact persisted snapshot. The manager checks current fences at model/tool boundaries; a captured revoked bundle yields `bundle_revoked`. A dispatched external action is not undone.

Runtime owns context attachment functions in proposed `bundle_admission.go`:

```go
func withBundleView(ctx context.Context, view bundlecontract.View) context.Context
func bundleView(ctx context.Context) (bundlecontract.View, bool)
func (s *Service) BeginDrain(ctx context.Context, operationID string) error
func (s *Service) WaitIdle(ctx context.Context, operationID string) error
func (s *Service) EndDrain(operationID string)
```

No public action or model input can forge the context. Child admission carries the parent's view explicitly through the existing authority checks. Non-bundle calls preserve existing behavior. Fence checks consult the storage/manager authority, not cached caller-provided booleans. Run revocation is sticky even if the toolkit later re-enables.

## C6 Skill and resource integration

TK-06 produces one compiled `BundleSkillSource` implementing existing skillsource.Provider through captured View. Stable provider ID is `vivy.bundle-skills`. Host ID encoding is architecture section 6. Calls without a captured run View are allowed only for authenticated management paths explicitly supplied a pinned current View; model calls cannot fall back to latest state.

Use existing `SkillOperations.ViewSkill(ctx, runID, name, relativePath)` to read toolkit resources. It already accepts a supporting-file path. Proposed `HostedSkillOperations` delegates toolkit List/View through SkillHost and reads only the target Skill body or files explicitly listed in Manifest.Resources; local mutation methods delegate to existing EinoSkillBackend and reject toolkit IDs with `read_only_component`. Bundle `BaseDirectory` remains empty so Eino cannot expose the managed store path. Skill view returns declared relative resource names and guidance to use `skill_view`; no new filesystem mount, public tool or broad FS grant is needed. Resource paths are relative to bundle root, never to cwd. Reject binary/non-UTF-8 and oversized text reads using existing SkillView bounds; binary artifact resources cannot be exposed through text SkillView.

All protected List/View paths, including control-plane list and explicit user invocation, share the stable ID mapping. User custom Skills can reference toolkit Skills only through independently authorized resolution; copying content is explicit. No toolkit Skill frontmatter populates DeclaredTools/Agent/Model/fork settings.

## C7 Mask changes

TK-07 extends maskcontract Definition/Metadata/Snapshot with `Origin string`, `Bundle *BundleProvenance`, and recommendation metadata where appropriate. `BundleProvenance` contains ReleaseRef and LocalID. Definition revision is release sequence, body digest remains existing DefinitionDigest. GenerationID is only the compiled origin for built-ins; bundle provenance must not impersonate it. Old snapshots derive builtin/custom origin from their existing fields. New prompt payload schema is 2; schema 1 remains readable.

Proposed methods:

```go
func (s *Service) CaptureFromView(ctx context.Context, sessionID domain.SessionID, view bundlecontract.View) (maskcontract.Capture, error)
func (s *Service) ListMasksFromView(ctx context.Context, req maskcontract.ListRequest, view bundlecontract.View) (maskcontract.Page, error)
```

These belong to the existing Mask service and internal resolver seam. Existing Capture/List wrappers pin current View for authenticated management; Runtime uses CaptureFromView with its reserved view. Recommendation fields are ComponentRefs plus projected Skill IDs/display/availability. They never write SkillHost scope or Tool grants. Validate same-bundle references at native validation and dynamic availability at inspection. For custom copies, preserve optional provenance/recommendations via explicit MaskCreate extension and paired nullable metadata persistence; do not quietly drop recommendations or treat them as grants.

Mask selection CAS and bundle catalog CAS share the run admission transaction. SetSelection uses active view and expected epoch. Disable retains unavailable selection; removal/update without selected Mask and uninstall fail `mask_in_use`. Copy action uses existing create operation identity with new custom UUID, immutable source provenance and current body bytes.

Extend custom create/update request digests to cover new provenance/recommendation values when present. Preserve the existing digest encoding for legacy requests with no new metadata, so previously recorded retries still match. Metadata-changing retries with the same operation ID must conflict rather than silently return an older copy.

## C8 Rules, Hooks and MCP

TK-08 creates captured Rule backend over existing Eino agentsmd seam and a prepared bundle hook chain. Rule synthetic paths are manager-owned keys; no real AGENTS.md mutation. Bundle Rules precede project files, reject interpreted @imports, preserve entire captured block and fail prompt budget before admission. Existing project-file loading remains separate.

ScriptHook gains explicit execution context fields for cwd/env and immutable resource root; preserve shell protocol and matcher semantics. Consent digest covers InstallID, ReleaseDigest, entry values, referenced binding names and artifact inventory. Never expose credential values. Reuse the existing process/execution owner; do not concatenate shell `cd` strings. Compiled governance precedes user hooks, then toolkit hooks sorted by BundleID/local ID. Only PreToolUse; revoked captured hooks fail the call, not skip it.

TK-09 validates native MCP entries into existing MCPServerConfig, using derived stable Name. Effective merge is a pure function rejecting duplicates before ReplaceServers. No Bundle entry is copied into user settings. Proposed Runtime functions:

```go
func PrepareBundleRules(ctx context.Context, view bundlecontract.View, face domain.Face, maxBytes int) (RuleBlock, error)
func PrepareBundleHooks(ctx context.Context, view bundlecontract.View, deps HookDependencies) ([]ToolHook, error)
func MergeMCPConfigs(user, bundled []MCPServerConfig) ([]MCPServerConfig, error)
```

RuleBlock contains ordered references, Body and Digest. HookDependencies contains existing process executor, binding resolver, configured timeout ceiling and live fence checker. Pure preparation resolves declarations/binding names only; credentials/processes/connections are resolved lazily at existing execution boundaries. A declaration with absent bindings or consent is inactive with a typed reason. Hooks and MCP each require separate consent, freshly bound to every new ReleaseDigest in v1. Revocation commits before acknowledging success.

## C9 Authenticated control operations

TK-10 exports `ActionProviders() []controlaction.Provider` from the compiled T1 `internal/modules/toolkits`. Action IDs use `vivy.bundles.<operation>`; owner/module ID is `vivy/toolkits`. This qualifies architecture section 17's short `bundle.*` names using the repository's existing action naming pattern.

| Suffix | Input beyond shared identity fields | Result | Effect |
| --- | --- | --- | --- |
| `list` | cursor, limit, state filter | metadata page, capabilities | read |
| `get` / `inspect` | install_id | detail / component and provenance diagnostics | read |
| `preview` / `previewGit` | source SourceSpec | PreparedSource | external-effect |
| `install` | preview_operation_id, expected_release_digest | Operation | write |
| `prepareUpdate` / `prepareUpdateGit` | install_id, source/ref override only within same origin | Operation with candidate/diff | external-effect |
| `enable` / `disable` / `uninstall` | install_id | Operation | write |
| `rollback` | install_id, release_digest | Operation | write |
| `operation.get` | operation_id | Operation | read |
| `operation.publish` | prepared_operation_id, candidate_digest, expected epoch/config revision | Operation | write |
| `operation.cancel` | operation_id, expected_operation_revision | Operation | write |
| `bindings.set` | install_id, expected_config_revision, typed binding names and local values/references | Operation with redacted binding state | write |
| `hook.arm` / `hook.disarm` | install_id, component_id, release_digest, declaration_digest, expected_consent_revision | Operation | write |
| `mcp.authorize` / `mcp.revoke` | same executable identity | Operation | write |
| `source.rebind` / `source.rebindGit` | install_id, new_source, expected_source_revision | Operation with explicit old/new origin | external-effect |

All mutations also require a new operation UUID and expected catalog epoch; canonical input digest is server-derived. Arming/authorization are restricted to authenticated direct control calls with no bound model Run, and ordinary Policy/Grant/approval remains authoritative. Do not duplicate the notebook policy exemption for convenience. Use distinct static local/fetch definitions: `vivy.bundles.preview` / `previewGit`, `prepareUpdate` / `prepareUpdateGit`, and `source.rebind` / `source.rebindGit`. The client chooses by explicit source kind; the server rejects mismatch. Both preview variants are external-effect because acquisition writes managed artifacts/operation state; source kind still restricts the permitted acquisition path. All names and schemas belong to one metadata inventory in the action module and must match TS client constants.

prepareUpdate and rollback return prepared candidate/diff records without publishing. `operation.publish` commits only the displayed prepared operation and candidate digest, subject to fresh CAS/consent checks. enable can prepare/publish its existing installed release directly after the explicit enable action. source.rebind stages a candidate with the proposed origin for review; publication records the source change and invalidates affected consent atomically. Raw previews do not create an enabled entry, and neither preview nor prepare performs executable effects from the toolkit.

Input limit 256 KiB, output limit 512 KiB; bodies larger than an output page use bounded component-detail pagination, never truncate silently. `get` returns metadata and summary; proposed read `component.get` accepts install_id, kind, local_id, expected_release_digest, optional declared resource path, offset and max_bytes; it returns bounded content/status, digest and next_offset. Default max_bytes is 32 KiB, maximum 64 KiB; enforce UTF-8 boundaries and return a safe next_offset. Revalidate release identity for every page. Its action Effect is read. Secret bindings are write-only; redact endpoint credentials and local private paths. A toolkit cannot call these operations through embedded text and gain authority. Pending operations use bounded polling through operation.get; no second event transport is introduced.

## C10 Toolkit UI contract

TK-11 creates authoritative sources at `plugins/vivy-toolkits-ui/ui/vivy-toolkits/src/`. Route `/toolkits`, navigation group `vivy`, order 65, title `Toolkits` / `工具包`, icon `wrench`, already present in `ui/src/plugins/host-icons.ts` and the SDK icon catalog. No new icon library or public icon name is required. Single route with stateful detail pane avoids assuming parameterized module routes. Query/selection is navigation state only; installation/activation state is fetched from backend.

Module ID `vivy/toolkits-ui`, Provider ID `vivy.toolkits-ui.sidebar`, `std/ui-extension@v1`, `repo:plugins/vivy-toolkits-ui`, i18n catalog `plugin.vivy/toolkits-ui.*` (existing SDK namespace, never UI product copy). UI consumes createModuleActionClient through @vivy/ui-sdk; no shell store or sibling Module imports. Backend management failures are typed; stale responses carry request sequence/selected installation guards.

Proposed files: `toolkit-client.ts`, `toolkit-state.ts`, `ToolkitPage.tsx`, `ToolkitLibrary.tsx`, `ToolkitDetails.tsx`, `ToolkitInstall.tsx`, `ToolkitOperation.tsx`, `ToolkitBindings.tsx`, `ToolkitConsent.tsx`, `ToolkitUpdate.tsx`, `index.tsx`. Consolidate only truly trivial presentation files if inspection shows no independent responsibility. `toolkit-state.ts` owns this page's disposable projection and pending request IDs, not domain truth or global session selection.

Install local directory via existing host picker if its contract exposes server-side directory selection; otherwise use a clearly labeled host path input validated by backend. Browser file uploads and browser-local path claims are not valid substitutes. Git source/ref is always available when backend reports support. No ZIP upload. Install result is disabled; enable and consent are separate explicit actions.

TK-12 adds Hooks/MCP/bindings and update-diff flows to the same page, then cross-links Mask/Skill owner pages via existing routes and supported navigation. No deep-link contract is invented silently; a link may open the existing owner page with a displayed component identity when that page has no selection query contract. Mask selection stays in existing Mask controls. Missing compiled component owners are rendered unavailable, not clickable fake routes.

## C11 Verification, economy, and change control

Planning performs document checks only. The commands in Story files are future implementation gates. Do not add/run implementation tests until the user authorizes implementation verification; if that permission is absent at execution, record verification blocked, never call a Story Done.

Reuse pinned Eino v0.9.13 Skill/agentsmd/ADK APIs and EinoExt MCP v0.0.9 through existing Runtime adapters. No dependency upgrade or new transport. Preparation cost is O(total declared bytes plus sorting the bounded inventory); hot-path view lookup is by immutable map, not repeated tree scans/hashing. Polling is only for active management operations. Fences use the existing storage authority; any later cache requires reliable invalidation proof. These are design costs, not measured performance claims.

All product-code deliveries require targeted owner checks, `just ci`, applicable PostgreSQL conformance, real UI smoke at the split server and iteration evidence. Repository justfile currently uses PowerShell conventions; execution must record whether `just ci` is available on the actual platform and never substitute an unreported partial gate. No production data paths may be used as fixtures. Schema rollback means transaction failure recovery and a compatible old artifact/epoch; never delete released migration history.

Changed contracts invalidate dependent plans/evidence through the index DAG. Do not mark a Story Ready on a documentation signature alone: its predecessor runtime producer and required evidence must exist. The current task authorizes planning only.
