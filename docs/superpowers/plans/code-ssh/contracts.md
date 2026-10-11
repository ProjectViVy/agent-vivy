# SSH-C1: shared worker and plugin contracts

**Revision:** SSH-C1, 2026-10-11. **Authority:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md)
§5-7. **State/dependencies:** [index](index.md). All symbols below are proposed;
none changes a Vivy SDK Port. SSH-01 implements the wire; SSH-02/03/06 implement
the worker; SSH-04/05/07 consume it. Host signatures remain blocked on SSH-00.

## Source and identity

- Module ID `vivy/ssh-workspace`, T2, source `repo:plugins/coding/ssh-workspace`;
  own Go module `agent-vivy/plugins/coding/ssh-workspace`.
- Independent Go module `agent-vivy/extensions/ssh-worker`, remote executable
  `vivy-ssh-worker`; `wire` is data/codec only. Plugin must not import `worker`,
  `workspace`, `receipts`, `process` or `cmd` implementation packages.
- Go baseline 1.26.4. Use explicit source/version pins in packaging; local module
  replacements are development conveniences, not sealed provenance.
- Protocol `vivy.ssh-worker/1`. JSON names are snake_case for every field in
  SSH-D1's Binding, CallScope, Request, Reply and Fault. Fields are required
  except `result` and `fault`; exactly one is present for terminal replies.
  A nonterminal reply has state and may include bounded diagnostic result.
- Operation key is `(binding digest, run_id, operation_id)`; a worker also rejects
  reuse of an existing operation identity under a different binding/digest.
  Read/query IDs are correlation only. Side effects require Host-issued scope.

Expected existing selectable Ports and sole Host consumers (all many-cardinality
at this baseline):

| Port | Consumer | Provider evidence |
| --- | --- | --- |
| `std/tool-world@v1` | `core/tool-host@v1` | Governed tool catalog and approved transport, SSH-04/05 |
| `std/context-source@v1` | `core/context-host@v1` | Bounded scoped data/Resolver; instruction semantics separately gated by H4, SSH-06 |
| `std/status-source@v1` | `core/status-host@v1` | Cached read-only snapshots, SSH-07 |
| `std/control-action@v1` | `core/action-host@v1` | Approved controls reenter existing bridges, SSH-07 |
| `std/ui-extension@v1` | `core/presentation-host@v1` | Removable page/navigation/chat.header, SSH-07 |

The relevant P3/P4/P6/P9 platform phases have recorded completion at the baseline;
recheck actual support/conformance before selection. Descriptor/Recipe Grant
bindings are finalized only with H2/H6 evidence. Current Actions allow rpc.client/
net.client and Status allows rpc.client; neither supplies raw Spawn. UI adds no
Grant. Listing a proc.spawn/net.client Grant on ToolWorld does not itself prove
target egress or per-operation remote filesystem authority.

## Codec and limits

Proposed `wire/types.go`, `codec.go`, `digest.go`, `payloads.go`:

```go
type Limits struct {
    FrameBytes, FileBytes, InlineOutputBytes int
    ArtifactBytes, ArtifactSetBytes, ArtifactChunkBytes int
}
func Encode(io.Writer, Request, Limits) error
func Decode(io.Reader, Limits) (Request, error)
func EncodeReply(io.Writer, Reply, Limits) error
func DecodeReply(io.Reader, Limits) (Reply, error)
func CanonicalRequestDigest(Request) (string, error)
func BindingDigest(Binding) (string, error)
func DecodePayload(method string, raw json.RawMessage) (any, error)
type Transport interface {
    RoundTrip(context.Context, Request) (Reply, error)
    Close(context.Context) error
}
```

Decode a bounded frame before method handling. Reject unknown/duplicate fields at
every schema level, invalid UTF-8, trailing JSON, wrong types, floats for integer
fields, invalid version/method and allocation over bounds. Canonical digests are
SHA-256 of deterministic typed JSON, including scope/binding/method/payload and
excluding request_digest. Preserve opaque strings and bytes; never normalize
file contents. The server validates digest before admission. Reply identity and
digests must match the request before a client accepts it.

Defaults: 2 MiB frame, 1 MiB file, 64 KiB inline output, 32 MiB artifact item,
128 MiB artifact set, 256 KiB artifact chunk. Count decoded bytes and encoded
frame separately; enforce min(Host, worker). Return bounded cursor output,
never truncate a mutation payload into a different valid operation.

## Closed payloads and results

The table specifies typed fields in `payloads.go`; encode/decode uses JSON tags
with the same snake_case names. `bytes` is base64 JSON; `digest` is lowercase
SHA-256 hex; paths/cwd are POSIX workspace-relative names. Requests cannot carry
credentials, arbitrary SSH options, target overrides or policy grants.

| Method | Request payload | Result / rule |
| --- | --- | --- |
| `hello` | `client_version`, `binding_digest` | Protocol/build digest, OS/arch, authenticated worker identity, root/default cwd, approved shell/PATH/redacted environment, methods, limits, isolation capability; mismatch refuses admission |
| `fs.list` | `path`, `cursor`, `limit` | Bounded entries with name/type/size; opaque cursor tied to binding |
| `fs.read` | `path`, `expected_digest` (optional) | Bytes, exact digest, size; over-limit or changed fails |
| `fs.search` | `path`, `pattern`, `cursor`, `limit` | Literal text matches, relative path/line and bounded excerpts; no arbitrary regex engine in V1 |
| `fs.write` | `path`, `expected_digest`, `bytes` | New digest; creation uses explicit `expected_digest=absent`; no overwrite with an omitted precondition |
| `fs.patch` | `path`, `expected_digest`, `old_text`, `new_text` | Exactly one old_text match required; new digest |
| `fs.multiedit` | `edits[]` of patch payload | Preflight all; per-file commit result with completed/unresolved paths and digests; not cross-file atomic |
| `process.execute` | `executable`, `args[]`, `cwd`, `env` (approved names only), `background`, `timeout_ms`, `isolation_required` | Job ID, state, bounded stdout/stderr, exit code when confirmed; executable/argv are subject to Host and worker profile |
| `jobs.list` | `cursor`, `limit` | Only binding-owned jobs; state and last-known timestamp |
| `jobs.output` | `job_id`, `stream`, `cursor`, `max_bytes` | Bounded chunk, next cursor, explicit gap/EOF; no false complete output |
| `jobs.cancel` | `job_id` | Host-issued new cancel operation; confirmed terminal state or unknown; receipt of original command remains linked |
| `operations.get` | `run_id`, `operation_id` | Read-only retained receipt; missing means unavailable, never permission to rerun |
| `artifacts.stat` | `path` | Exact relative path/digest/size/media type/binding reference |
| `artifacts.read` | `path`, `expected_digest`, `offset`, `max_bytes` | Chunk under unchanged digest; changed/unavailable fails, never substituted bytes |
| `lease.renew` | `connection_id` | Renew the owned connection lease only; cannot extend another owner |
| `close` | `connection_id` | Stop new admission; confirmed cleanup summary or unknown jobs; idempotent |

`hello` establishes one server-validated binding and connection ID for this
stdio instance. Every request must match it. Worker startup accepts fixed
operator-approved root/profile/state arguments; wire cannot widen them. Allow
one pending request per connection in V1. A foreground command returns a job
handle promptly; client polling supplies foreground waiting/output, so renew,
cancel and receipt queries cannot deadlock behind a long command. Worker output
is framed replies only; diagnostics go to bounded stderr.

## Worker-owned boundaries

Proposed interfaces are implemented once and used by the worker dispatcher:

```go
type Receipt struct {
    Scope CallScope
    BindingDigest, RequestDigest, State string
    Result json.RawMessage
    Fault *Fault
}
// receipts.Store (uses wire.Receipt; aliases are not a second contract)
func OpenStore(stateDir string) (*Store, error)
func (s *Store) Admit(Request) (Receipt, bool, error) // true only for new intent
func (s *Store) MarkStarted(Request) error
func (s *Store) Settle(Request, Reply) error
func (s *Store) Get(Binding, CallScope) (Receipt, error)
func (s *Store) Recover() error
// workspace.Workspace
func Open(root, stateDir string, limits Limits) (*Workspace, error)
func (w *Workspace) Handle(context.Context, Request) (Reply, error)
func (w *Workspace) OpenCwd(relative string) (*os.File, error)
func (w *Workspace) Close() error
// process.Manager
func NewManager(*receipts.Store, *workspace.Workspace, Profile, Clock) *Manager
func (m *Manager) Handle(context.Context, Request) (Reply, error)
func (m *Manager) Close(context.Context) (CloseResult, error)
```

These snippets omit package qualifiers inside each defining package. `Profile`,
`Clock` and `CloseResult` are process-owned types: Profile is fixed approved
executable/env/isolation/lease policy; Clock supplies Now and cancellable timers;
CloseResult lists confirmed and unknown job outcomes. Context cancellation is a
request to clean up, not evidence cleanup succeeded. Unknown receipts are never
automatically promoted or replayed. Store failure stops new side effects.
OpenCwd returns a securely resolved directory handle owned by the caller until
process startup completes. The launcher must preserve that handle's directory
identity, not re-resolve a model path string after validation.

## Plugin-owned boundary and unresolved Host producer

Proposed `transport.go` consumes `toolworld.Host.Spawn(ctx, SpawnSpec) (Proc,error)`
only after the verified Host path authorizes profile/egress. `SSHTransport`
implements `wire.Transport`. `manager.go` owns that connection and binding:

```go
func NewTransport(ApprovedTarget, toolworld.Host, wire.Limits) *SSHTransport
func (m *Manager) Invoke(context.Context, wire.Request) (wire.Reply, error)
func (m *Manager) Snapshot() Snapshot
func (m *Manager) Close(context.Context) error
```

ApprovedTarget is a validated immutable target revision, root, worker digest and
opaque auth/known-hosts profile produced by the accepted H1/H2 path. It is not a
browser/model DTO. Snapshot is a copied read model, never a connection probe.
No factory signature for multi-Port shared Manager is invented here: H5 must
prove the real producer. No public CallScope/admission/resource API is invented:
H1-H4 must supply exact existing signatures or a separately approved change.
SSH-00 updates all affected Stories before they become Ready.

Error codes used by both sides: `invalid_request`, `denied`, `unavailable`,
`binding_changed`, `version_mismatch`, `digest_conflict`, `stale_file`,
`ambiguous_patch`, `limit_exceeded`, `isolation_unavailable`, `unknown_outcome`.
Fault Retry is closed to `never`, `query_receipt`, `fresh_read`. Public messages
contain no secret/auth bytes. Preserve safe internal cause chains separately.
