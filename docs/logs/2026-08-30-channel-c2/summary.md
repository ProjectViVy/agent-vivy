# CH-C2 — SDK `seam: channel` + empty registry + envelope configuration (summary)

Date: 2026-08-30. Branch: `feat/channel-c2` (cut from `feat/channel-c1` c811d9f,
in an independent worktree).
PLAN: `docs/plans/channel-epic/CH-C2.md`. Contract:
`VIVY-CHANNEL-PACK.md` §4/§8/§9/§10/§11.

## What changed

The product window now exposes the `seam: channel` seam: authors can write channel
plugins, verify accepts them, and pack can overlay a plugin with its **own go.mod**
into `Register()`—but the default body is still `Register() = nil`, there is no Host,
and it cannot run yet.

1. **sdk/plugin**: `SeamChannel`; five Grants (`channel.poll` /
   `channel.webhook` / `channel.listen` / `channel.a2a` / `secret.read`; the
   vocabulary accepts all of them, while verify enforces seam restrictions). New file
   `sdk/plugin/channel.go`: every symbol of the contract §9.3 `Channel` /
   `ChannelEnv` interfaces is implemented (`Send` returns `([]string, error)`;
   `HTTP()` is an outbound client with no Listen; `Secret` is fail-closed);
   typed `InboundMessage` / `OutboundMessage` / `Part` envelopes (text /
   media-ref / structured; **maps are forbidden as the primary contract**); nine
   reserved capability slots (MediaStore, Typing, MessageEditor, Placeholder,
   MediaSender, WebhookHandler, StreamingCapable, TaskLifecycle, PipeServer, with zero
   methods plus retained comments, asserted only by the C3 Host).
2. **verify**: dispatches by seam. `seam: channel` ⇒ forbid `tools`, require
   grants ⊆ {channel.poll, secret.read} (webhook/listen/a2a are rejected in this
   batch), require a `channel` object with `transport: "poll"`; non-channel seams
   may not claim channel-family grants. AST-level bans add `net.Listen` /
   `http.ListenAndServe[+TLS]` (Listen belongs to the Host); the existing eino /
   internal / os.Open / exec bans continue to apply to channel. New fixtures:
   `bad-channel-tools` / `bad-channel-listen` / `bad-channel-grant`; positive
   fixture `TestVerifyFakeChannel`.
3. **pack independent module**: if a `--with` plugin has its own go.mod, parse its
   module path as the import path for generated `Register()`; build with a **two-file
   overlay** (generated zz_register.go + a root go.mod copy with
   `require <mod> v0.0.0` + `replace <mod> => <abs>` appended), leaving the real
   tree untouched. `TestPackFakeChannelStandaloneModule` runs a real `go build` and
   asserts that both live `zz_register.go` **and** live `go.mod` are byte-for-byte
   unchanged.
4. **pluginhost**: `Adapt` explicitly skips `SeamChannel` (a test channel stub with
   non-empty Tools() proves this is not an accidental empty result)—channel never
   enters the tool table.
5. **config**: `channels:` envelope skeleton (map<name> →
   `{enabled, allow_from, token_env, settings}`, with settings opaque as a
   `yaml.Node`); structural validation covers slug names, `token_env` matching the
   env-key pattern (D-010 copy), non-empty `allow_from` entries, and rejection of
   `*`; empty `allow_from` is allowed in config (refusing Start belongs to the C3
   Host); unknown keys inside settings do **not** error (tests pin the interaction
   between strict decoding and opacity). Validation of unknown names against
   `Register()` is left to C3 per PLAN.
6. **PLUGIN-SPEC §4 symbol-level alignment** (within the scope authorized by CH-C2 §5,
   with no contract semantic change): Grant constants + Channel/ChannelEnv signatures +
   one sentence on typed envelopes.

## Explicitly not done

- No `internal/channelhost`, Session mapping, or `Service.Run` wiring (all C3).
- verify cannot type-check "must implement Channel" (the AST/manifest layer cannot do
  this)—the C3 Host assertion is the backstop, recorded in §0.1.
- `inspect` / `generation.json` classification by seam
  (name/version/seam/grants/transport/…) was not done; it belongs to a later slice.
- No real platform SDK was introduced; the independent module's transitive dependency
  closure (merging require/go.sum into the overlay) is left for the real C4 package.
- Submitted `zz_register.go` still `return nil`; `go.mod` /
  `go.sum` unchanged; not pushed.
