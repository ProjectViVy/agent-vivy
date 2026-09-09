# CH-C2 — SDK `seam: channel` + Empty Registry + Envelope Configuration

## 1. Identity

| | |
|---|---|
| ID | CH-C2 |
| Stage | B Species Window |
| Person-days | 2 |
| Milestone | M-CH1 |
| Dependency | CH-C1 |
| Successor | CH-C3 |
| Branch | `feat/channel-c2` |
| Contract | §4, §9, §10, C2; `VIVY-PLUGIN-SPEC.md` SeamChannel |

## 2. Goal

An author can write a `seam: channel` Go package, have it accepted by `vivy-sdk verify`, and have `pack` overlay it into `Register()`, while the default body remains `Register() = nil`. There is no Host yet, so it cannot run yet.

## 3. Current State

- `sdk/plugin/plugin.go`: Seam currently has only tool / tool-world / provider; Grant has only fs.read/write; Plugin must have `Tools()`.
- `sdk/internal/verify.go`: blocks eino/internal/os.Open; no channel rules.
- `sdk/internal/pack.go`: imports `plugins/<name>` by species module path.
- `internal/pluginhost/host.go`: `Adapt` unconditionally iterates `Tools()`.
- `internal/generated/plugins/zz_register.go`: `return nil`.
- `internal/config`: no `channels:` envelope.

## 4. Target Structure

```text
sdk/plugin
  SeamChannel
  GrantChannelPoll / Webhook / Listen / A2A / GrantSecretRead
  Channel { Name Seam Grants Start Stop Send }
  ChannelEnv { Secret HTTP PublishInbound Media }
  InboundMessage / OutboundMessage / Part   # First shape of the slot; see contract §8
  Optional capability interface types (empty implementations are sufficient; C3 Host is the first to assert them)

verify
  seam:channel → zero tools, must behave like Channel, no Listen, no webhook/listen/a2a grant (this batch)
  testdata/fake-channel/   independent small module, no bloated SDK

pack
  When --with points to a plugin with an independent go.mod, Register() imports that module
  Default zz_register.go remains nil

pluginhost.Adapt
  SeamChannel → skip (must not become a tool)

config.channels
  Envelope: enabled, allow_from, token_env, settings (opaque)
  Full startup failure for unknown names can wait for C3; this slice must at least parse and validate the structure
```

Do not split the Plugin shape of `hello-fs`. Do not churn the tool ABI for channel support yet. `Register()` still returns `[]plugin.Plugin`; a channel plugin implements both `Plugin` (with empty Tools) and `Channel`.

## 5. File Inventory

**Modify/Create**

- `sdk/plugin/plugin.go` (and split files if it becomes too large)
- `sdk/internal/verify.go` + `verify_test.go` + testdata
- `sdk/internal/pack.go` + `pack_test.go`
- `sdk/internal/testdata/fake-channel/` (independent go.mod, no third-party SDK)
- `internal/pluginhost/host.go` + tests: channel does not appear in the tools table
- `internal/config` envelope skeleton + parse tests
- `VIVY-PLUGIN-SPEC.md` if the implementation needs symbol-level alignment with the draft (do not change contract semantics)

**Do Not Touch**

- `internal/channelhost` (does not exist yet; create it only in C3)
- The real `plugins/telegram` package (C4)
- Adding telego to a species' `go.mod`
- Submitting a non-empty `zz_register.go`

## 6. Steps

1. Extend Seam/Grant; have `Valid()` cover the new values.
2. Envelope types: use structs for parts; prohibit `map[string]string` as the primary contract.
3. `Channel` / `ChannelEnv` interfaces. ChannelEnv.HTTP is an outbound client; there is no Listen.
4. verify branch: follow the `bad-eino-import` style and add `bad-channel-tools` and `bad-channel-listen`.
5. testdata fake-channel: independent go.mod, with `replace` pointing to the species sdk/plugin.
6. pack: overlay fake-channel; assert that the generated file imports that module path; restore afterward or test in a temp overlay; **do not submit a non-empty Register()**.
7. Make Adapt skip channels.
8. Test config-envelope decoding.
9. `just ci`.
10. Log to `docs/logs/YYYY-MM-DD-channel-c2/`.

## 7. Acceptance

- `vivy-sdk verify sdk/internal/testdata/fake-channel` passes.
- A channel manifest with tools fails.
- Source containing `net.Listen` fails.
- The `go test ./...` import graph in `just ci` contains no telego.
- `zz_register.go` in the submitted tree remains `return nil`.
- `pluginhost.Adapt([]Plugin{fakeChannel})` has length 0.

## 8. Prohibitions

- Implement the Host / Session mapping / Service.Run wiring.
- Enable `channel.webhook` / `listen` / `a2a` grants in this batch's recipe.
- Change the ADR-015 "empty registry" semantics for an independent module.

## 9. Risks and Rollback

- pack may be larger than expected for an independent go.mod: this slice permits **connecting only the fake-channel module**; leave the real telegram module to C4.
- A Plugin implementing both Tools()+Channel may alarm old code: Adapt skip is a hard acceptance criterion.
- Rollback: revert; there is no runtime behavior.

## 10. Handoff

Next AGENT: [CH-C3.md](CH-C3.md). The Host consumes this slice's Channel/Env/envelope symbols; do not rename them in C3.

> **DONE 2026-08-30** — Branch `feat/channel-c2`; symbols finalized in `sdk/plugin/channel.go` (`Channel`/`ChannelEnv`/`InboundMessage`/`OutboundMessage`/`Part` + 9 reserved capability slots). C3 directly consumes these names; §8 still needs run_id/task_id slots and Delete/Reaction/HealthChecker/ListenHandler (TODO §0.1 CH-C2-N2). Filing: `docs/logs/2026-08-30-channel-c2/`.
