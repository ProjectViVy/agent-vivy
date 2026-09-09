# CH-C2 — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: the body is unchanged, but authors have a new seam

- Daily `vivy.exe` / `http://127.0.0.1:3015` behavior is **unchanged**: the default
  body's `Register()` is still empty, there are no ears, and no new configuration is
  required.
- The change is in the **authoring and packaging experience**:
  `seam: channel` is now a first-class plugin-manifest citizen.

## Human-verifiable points

1. **The gate is green**: `just ci` from the repository root exits 0.
2. **A channel plugin can be accepted**:
   `go run ./sdk verify sdk/internal/testdata/fake-channel` → `ok`. fake-channel is
   a minimal sample with its own `go.mod` (Start/Stop/Send/PublishInbound, no tools,
   and only `channel.poll` + `secret.read` grants).
3. **An invalid Consumer is rejected**:
   - A manifest with `seam: channel` plus `tools` → verify fails: "seam channel forbids
     tools"
   - `net.Listen` in source → verify fails: "opens a listen socket (Listen belongs to
     the kernel ChannelHost)"
   - A manifest claiming the `channel.webhook` grant → verify fails: "not allowed in
     this batch"
4. **channel never enters the tool table**: `pluginhost.Adapt` returns zero tools for
   seam:channel (the test stub deliberately includes one tool to prove the skip is
   explicit).
5. **A plugin with an independent go.mod can be packaged**:
   `pack --with sdk/internal/testdata/fake-channel` produces a buildable generation;
   the real tree's `go.mod` and `zz_register.go` remain byte-for-byte unchanged
   (test assertion).
6. **Configuration grows an envelope, not an organ**: a
   `config.yaml` entry of
   `channels: {telegram: {enabled, allow_from, token_env, settings}}` passes parsing
   and structural validation; lowercase `token_env` and `allow_from: "*"` are
   rejected; future fields inside `settings` do not error (the kernel treats it as
   opaque). Validation of names that do not match this generation's body lands with
   the C3 Host.

## Explicitly not part of this slice's acceptance

- An ear can send and receive messages → after CH-C3 (Host + fake-plugin loop).
- Seeing channels in Settings/inspect → CH-C5.
- Real Telegram/DingTalk → CH-C4/C6.

## Rollback

Revert this branch: the default body has zero behavior change, and the risk surface is
limited to the new SDK types and config section (both optional).
