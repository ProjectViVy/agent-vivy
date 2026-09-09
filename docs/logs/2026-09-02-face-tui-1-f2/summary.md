# FACE-TUI-1 F2 — seam-face SDK contract + kernel FaceHost RunFace + built-in faces/headless organ + pack `--face`

Date: 2026-09-02. Scope: the F2 slice of FACE-TUI-1 (implementation of the
VIVY-FACE-PACK.md PR 3 contract). F1 (`2026-09-02-face-tui-1-f1`) and the §14
four-question decision (`2026-09-02-face-pack-14-rulings`) are prerequisites; this slice
does not change either conclusion.

## Deliverables

**SDK contract (`sdk/plugin`)**
- `SeamFace Seam = "face"`: a face organ is a "mouth" hosted by the kernel FaceHost — it
  is a control-plane client and never a model tool (§6).
- Face-family grants: `tty` (terminal I/O), `argv` (command-line arguments), and
  `rpc.client` (call the control plane through `FaceEnv.Call`).
- `face.go`: `Face` (Kind/Run), `FaceEnv` (Call/OnEvent), `FaceOptions`
  (Prompt/ContinueNewest/Out/Err — the launcher owns stdout/stderr; the organ may not
  open its own), `FaceResult{Status}`, and `FaceConstructor`.

**SDK validation (`sdk/internal`)**
- Manifest: the `face` envelope (kind/listen) plus `checkFaceManifest`: zero tools,
  `kind ∈ web|tui|headless`, `listen` must be false (in this batch; listening is the
  face's own effect), and grants limited to the face family; a non-face seam declaring a
  face object is an error.
- Inspect: constructor rules dispatch by seam — face organs require
  `func New(plugin.FaceOptions) plugin.Face` (`hasNewFace`); other seams still require
  `func New() plugin.Plugin`.
- Verify: pass the seam into checkSources.

**Kernel (`internal`)**
- `internal/generated/face/zz_face.go`: the default registry returns nil — the committed
  body has no face organ, and `vivy run` uses the existing kernel headless loop.
- `internal/app/facehost.go`: `RunFace(ctx, cfg, ctor, opts)` — gateway-less composition
  (WithoutEars+WithoutGateway) + `DialControl(net.Pipe)` + the `faceEnv` adapter
  (Call→Peer.Call; OnEvent registers notification callbacks; server-side ID requests return
  null). The organ sees only FaceEnv, not App.
- `cmd/vivy/run.go`: when `face.Register()` is non-nil, use `RunFace` (organ side); otherwise
  leave the existing `RunHeadless` path unchanged. Exit-code mapping
  completed=0/cancelled=2/other=1 is consistent across both paths.
- Add the `Face` field to `domain.AssemblyRecipe`.

**`faces/headless` organ (standalone module)**
- `faces/headless/`: its own go.mod (`module example.com/vivy/faces/headless`,
  `replace agent-vivy => ../..`), with zero third-party dependencies. Manifest: seam face,
  grants tty/argv/rpc.client, and `face{kind headless, listen false}`.
- `headless.go` speaks only control-plane JSON-RPC: initialize → session resolution
  (`--continue` takes the latest from session/list; otherwise session/create, title = prompt
  truncated to 60 runes) → turn/start (face=headless) → run/subscribe (after_seq 0,
  replaying events that occurred before subscription) → event rendering (model.delta→Out,
  model.completed adds a line, tool.started/failed→Err,
  run.failed/cancelled/completed terminal states). **§14④**:
  tool.approval_required / user.question_required → loud notification + run/cancel
  (durable cancelled terminal state); do not wait, yolo, or bypass HITL. Semantics match
  kernel headless.go line by line.

**Pack (`sdk/internal/pack.go`)**
- `--face <organ>`: at most one ("one mouth per generation", §14③);
  resolveFaceDir (conventional candidates under faces/) + verify (seam must be face) + a
  build-time overlay of `internal/generated/face/zz_face.go` (generates
  `Register() plugin.FaceConstructor { return <pkg>.New }`).
- The faces module is a standalone module → reuse the existing -modfile merge path
  (`pack.mod`/`pack.sum` are temporary pairs; live go.mod/go.sum are untouched).
- Artifact adds a `face` record (name/version/kind/grants/source_ref/tree_hash);
  `recipe.face` is recorded.

**CI**
- justfile: fmt-check adds `faces`; plugin-ci now iterates over both module roots,
  `plugins/` + `faces/`.

## Explicitly not done (left to F3 and later slices)

- F3: interactive TUI organ in `faces/tui` (bubbletea, etc.) — a separate slice.
- faces/web: the web face remains the gateway-embedded UI form; this slice does not migrate
  it.
- `face.listen: true` and webhook/listen channel-style face transports: explicitly rejected
  in this batch.
- Fine-grained runtime arbitration for face grants: the verifier gates the wording in this
  batch; FaceEnv has a single implementation.

## Review points

- The committed body (packed without `--face`) is behaviorally unchanged: the default
  registry is nil → the original RunHeadless path; internal/generated/face/zz_face.go is
  the manually maintained default, and pack replaces it only in a build overlay, never in
  the worktree.
- Zero tenant-Journal contact: smoke runs in an isolated `%TEMP%` directory (see
  verification.md).
