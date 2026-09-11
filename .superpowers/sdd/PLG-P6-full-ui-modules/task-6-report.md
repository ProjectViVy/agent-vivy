# Task 6 report: real full-access UI fixture

The selected T2 fixture module is `fixture/full-ui`. Its exclusive root
observes the live Face store, while its ordered extension registers the
navigation item and `/dashboard` route, global style/theme, and deterministic
cleanup. The root invokes the typed `module.action.invoke` client against the
generated ActionSet.

The Playwright test is opt-in through `VIVY_FULL_UI_URL`. It compares the
ordinary generated UI at `127.0.0.1:8799` with the selected Vite split pair
at `127.0.0.1:3015`, checks root/route/style/theme/live state, invokes the
real authenticated action, rejects forged approval/Trust fields, checks
Inspect for the absence of UI Grant/permission surfaces, and navigates back
to the default origin to check that no selected DOM state leaks.

## Exact split-pair setup

Run from the repository root. These identities match the checked-in fixture:

```sh
SOURCE_HASH=c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc
LOCK_HASH=117e929ecf1f2ee5dc1673cfdede4f16e1c4471e9c30e836b6a5be3b9075b2d0
ASSET_HASH=265191fd352c9d55590dbc4a90fcedb05e411ebf7ac55889bb613afaaa784531
RECIPE=$(mktemp)
ARTIFACT_PARENT=$(mktemp -d)
ARTIFACT="$ARTIFACT_PARENT/full-ui"
cat >"$RECIPE" <<EOF
apiVersion: vivy.generation/v1
profile: minimal
modules: [vivy/kernel, vivy/tool-host, fixture/full-ui]
sources:
  fixture/full-ui: {ref: file:full-ui-module, sha256: $SOURCE_HASH}
exclusive:
  std/ui-root@v1: fixture/full-ui
order:
  std/ui-extension@v1: [fixture/full-ui]
grantApprovals:
  - {module: fixture/full-ui, name: rpc.client}
ui:
  sdkVersion: 1.0.0
  root:
    id: fixture.full-ui.root
    moduleId: fixture/full-ui
    port: std/ui-root@v1
    entry: ./ui/src/index.tsx
    export: root
    sourceHash: $SOURCE_HASH
    dependencyLockHash: $LOCK_HASH
    assetHash: $ASSET_HASH
  extensions:
    - id: fixture.full-ui.extension
      moduleId: fixture/full-ui
      port: std/ui-extension@v1
      entry: ./ui/src/index.tsx
      export: extension
      sourceHash: $SOURCE_HASH
      dependencyLockHash: $LOCK_HASH
      assetHash: $ASSET_HASH
EOF
GOFLAGS=-buildvcs=false go run ./sdk pack --recipe "$RECIPE" --output "$ARTIFACT" --source sdk/internal/testdata/full-ui-module
```

Stage the packed Assembly and selected source below `ui/` (Vite's strict
source boundary), then start the packed backend and Vite in separate shells:

```sh
PAIR_UI=$(mktemp -d ui/.e2e-full-ui.XXXXXX)
cp "$ARTIFACT/ui-assembly.ts" "$PAIR_UI/assembly.ts"
mkdir -p "$PAIR_UI/ui"
cp -R sdk/internal/testdata/full-ui-module/ui/. "$PAIR_UI/ui/"
```

```sh
SELECTED_WORKDIR=$(mktemp -d)
mkdir -p "$SELECTED_WORKDIR/workspace"
cat >"$SELECTED_WORKDIR/config.yaml" <<EOF
server:
  addr: "127.0.0.1:8787"
storage:
  backend: sqlite
  sqlite:
    path: "$SELECTED_WORKDIR/state.db"
providers:
  active: openai
  bundle_dir: "$PWD/fixtures/provider"
  openai:
    env_key: OPENAI_API_KEY
    default_model: gpt-4o-mini
runtime:
  workspace_root: "$SELECTED_WORKDIR/workspace"
  stream_buffer: 256
  max_event_payload_bytes: 65536
tools:
  enabled:
    - echo_info
    - write_note
    - ask_user
  approval:
    expiration: 5m
EOF
VIVY_CONFIG="$SELECTED_WORKDIR/config.yaml" "$ARTIFACT/vivy"
```

```sh
VIVY_BACKEND_ADDR=http://127.0.0.1:8787 \
VIVY_UI_ASSEMBLY_ENTRY="$PWD/$PAIR_UI/assembly.ts" \
pnpm --dir ui dev
```

With both processes ready, run:

```sh
VIVY_FULL_UI_URL=http://127.0.0.1:3015 \
pnpm --dir ui exec playwright test e2e/plugin-full-ui.spec.ts
```

No fixture package installation or network fetch is required; the fixture
lockfile contains only the local SDK link and exact React 19.2.8.

## RED / verification status

The default-origin assertions were written before the fixture and selected
Assembly existed. The selected phase skips unless `VIVY_FULL_UI_URL` is set,
so the split pair and Playwright run were deferred in this coding pass. The
fixture Go package, source hash, and a Pack/build smoke (with
`GOFLAGS=-buildvcs=false`) were checked; the split-pair browser run and
final running-binary interaction remain unverified.

The compiler's implicit ActionHost-use mark, action Port evidence, and
repository-path source snapshot exception are small build-boundary support
needed for the selected T2 provider to reach generated ActionSets and the real
authenticated RPC.
