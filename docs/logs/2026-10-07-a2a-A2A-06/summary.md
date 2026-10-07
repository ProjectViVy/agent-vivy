# A2A-06 Summary — official SDK Module, Recipe and end-to-end artifact acceptance

## .1 feat(a2a): map official protocol transport onto native TaskHost — d90bdcd1

- `plugins/a2a-server`: custom `RequestHandler` over
  `a2asrv.NewJSONRPCHandler` (a2a-go v2.6.0): `SendMessage` /
  `SendStreamingMessage` / `GetTask` / `ListTasks` / `CancelTask` mapped
  onto the native TaskHost contract; four push methods and the extended
  card return explicit unsupported.
- Mapping table tests for every design §7/§11 state/error: user role +
  text-only parts, HistoryLength -1/0/omitted/positive semantics,
  message-ID dedup retries, context consistency; invalid requests show
  zero Submit calls.
- Card advertises only configured compiled capabilities, bearer scheme
  and safe skills; `supportedInterfaces[].url` carries the `/a2a` suffix
  (the only mounted RPC route).
- Stream sends lead with the snapshot, terminate on both interrupted
  states, and emit full-replacement artifacts; no wire cursor extension.

## .2 feat(a2a): compose optional server module and native client smoke — 410fab48

- `module_v1.go` provides `std/channel@v1` id `vivy.a2a`, requires
  `core/channel-host@v1`; `recipes/a2a.vivy.yml` adds the module
  explicitly (opt-in; default Assembly and production dependency closure
  untouched).
- App composition `ChannelModuleIDs` wires provides-id → module; the
  listener mounts only with the `channel.a2a` grant binding.
- Code-face admission fix: `RunOptions.Face = domain.FaceCode` for
  channel tasks so remote peers skip persona-onboarding gating.
- `TestA2AModuleComposition`, `TestA2AApprovalIsLocalOnly`,
  `TestA2AListenerWithoutModule`, `TestA2ANativeServicePath` (7
  subtests) and env-gated `TestA2AArtifactClientSmoke`: 401 without
  bearer, stream→COMPLETED with "artifact answer" in history, identical
  -payload retry returning the same task.

## .3 test(a2a): verify selected and omitted server artifacts — (this commit)

- `TestA2APackedArtifactSmoke` (sdk/internal): packs
  `recipes/a2a.vivy.yml`, launches the artifact binary against a
  scripted OpenAI-compatible model that requests a governed `list_dir`
  tool call before final text, then drives the official client smoke —
  no echo-only success.
- `TestA2ASelectedAndOmittedArtifacts`: selected artifact carries
  `projectvivy/a2a-server` module + `channel.a2a`/`secret.read` grants +
  channel binding + `a2a-go` binary dependency (`go version -m`
  evidence); default/minimal artifacts physically omit the module,
  route, listener, settings and SDK dependency; plugin tree scans clean
  for `net.Listen`/`ListenAndServe` and `internal/*` imports.
- Fix commit: `module_v1.go` `RequestedGrants` corrected
  `channel.poll` → `channel.a2a` (descriptor/yaml/recipe agreement,
  asserted in `TestA2AModuleComposition`).
- `chore(conformance)` repin: root `go mod tidy` drifted nested modules;
  tidied `faces/tui`, `plugins/{discord,qq,vivy-persona}`, restored the
  generated-workspace requires (`faces/{headless,tui}`,
  `plugins/{governance,coding/lsp}`), repinned all affected
  `vivy-module.yaml` + `releaseSuiteCases` + internal digest +
  regenerated `ui/src/generated/assembly.ts` to fixed points.
