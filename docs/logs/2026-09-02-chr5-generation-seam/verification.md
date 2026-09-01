# Verification — CH-R-5

| Check | Command | Result |
|---|---|---|
| gofmt | `gofmt -l sdk/internal` | first run flagged `pack.go` (struct literal alignment) → `gofmt -w` → clean |
| Build | `go build ./...` | ok |
| Vet | `go vet ./sdk/...` | ok |
| Pack tests (real go builds) | `go test ./sdk/internal/ -run 'TestHashPluginTree\|TestPack' -count=1` | ok — `agent-vivy/sdk/internal 27.8s` |
| Kernel gate | `just ci` | see below (run in background, exit code tail-checked) |

## Test coverage added

- `TestHashPluginTree`: determinism (hash twice equal), 64-hex shape,
  one-byte content change → different digest, nested files included.
- `TestPackFakeChannelStandaloneModule`: `art.Plugins` exactly one entry —
  name/version/seam `channel`, grants `[channel.poll secret.read]` exact,
  transport `poll`, `source_ref == file:<abs testdata dir>`, 64-hex
  tree_hash (channel contributes no tools — pre-existing assertion kept).
- `TestPackHelloFSWritesArtifactAndLeavesLiveRegister`: hello-fs entry with
  seam `tool-world`, grants `[fs.read]`, **empty** transport (off the
  channel seam), file: source ref, 64-hex tree_hash.
- `TestPackTwoStandaloneModules`: both `plugins[0]=telegram` and
  `plugins[1]=discord` carry the full channel projection with tree_hash —
  order preserved alongside `recipe.plugins`.
