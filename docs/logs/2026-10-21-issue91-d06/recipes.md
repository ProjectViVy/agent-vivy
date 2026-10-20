# Executed Recipe and fixture identities

Candidate cc74e7d8ea6474190c8c182d42f51496c4d7fccc; all rows actually packed and passed nonexecuting artifact
inspection. Inputs are the named checked-in files/test-owned fixtures at this
commit, with new t.TempDir output directories for each pack. IDs are opaque
pack allocations, not content hashes. All packs use fresh t.TempDir outputs;
pack supplies identity and overrides authoring values. Executable rows use default
pack options with the named Recipe; DIVA supplies Target=shared. Go-host uses
HostPackage=./cmd/gohost, HostAssets=agent-diva-gui/dist and its test-created
native source/lock from goHostUniverse in sdk/internal/go_host_test.go. The ten checked-in recipes/ files are all
covered; no required variant is unavailable. DIVA uses its supported shared target.

| Recipe / input | Target | Generation ID |
| --- | --- | --- |
| sdk/internal/testdata/full-ui.vivy.yml + --source sdk/internal/testdata/full-ui-module | executable | `80d73f15a2ec10531fdf3029c23559f1` |
| recipes/default.vivy.yml | executable | `a1d9467ae8e56911086054f441085a18` |
| recipes/lite.vivy.yml | executable | `a287b2675c42fad8505c5f8b2e9f1275` |
| recipes/masks-selected.vivy.yml | executable | `ec16668e75bc746e3e00fa3eed3027e4` |
| recipes/masks-omitted.vivy.yml | executable | `b1bfff84c50493cd22a9ae4066c66f76` |
| recipes/masks-backend-only.vivy.yml | executable | `4f39f32cb9d7ace327fe6cdc9f853210` |
| recipes/minimal.vivy.yml | executable | `346132a734bc8b776f334ccb67c65f48` |
| recipes/headless.vivy.yml | executable | `febb0b6ce41cca244b4edcefddb30243` |
| recipes/vivy-code.vivy.yml | executable | `e3fdd303582ffa11c30d9f32c648b99c` |
| recipes/scx.vivy.yml | executable | `afa7a9c770096999dc940907e983d7d4` |
| TestPackLinksExternalStandaloneModule: acme/greet @ file:test | executable | `bbcdc01269889d48829bee78280f8a06` |
| recipes/diva.vivy.yml | shared | `3c336bfec1b101ab285b60750e97238e` |
| sdk/internal/testdata/go-host (TestPackGoHostSealedExternalArtifact) | go-host | `0d318f296bbb2c238b5305d7bf57645f` |

Final full-SDK tool-world/native source receipts:

```text
frontend_v1_test.go:799: fixture=hello-fs target=executable generation=eb3489cbd92816aaf75567a59d021802
frontend_v1_test.go:799: fixture=lsp target=executable generation=07c3c9249ad3856c0956f48f62235a61
go_host_test.go:301: go-host native source=https://example.invalid/go-host commit=df604fe8acf46adf27e335ef28c339d05a57eeec
```

The dynamic tool-world Recipes are exactly the literals in
TestPackSelectedToolWorlds; the standalone acme/greet fixture is exactly the
source/ref/Recipe produced by its named test. The public host API external-
consumer fixture separately compiles through native Go module integrity.
Plugin-v1 cases.json is a static compiler accept/refuse corpus, not executable
Module source. The generated UI default/extension/replacement fixtures run with
explicit authoring-test IDs, compile/typecheck and structural Inspect; they are
covered by the actual SDK assembly suite. No absent executable fixture is claimed.
