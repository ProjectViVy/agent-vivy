# 2026-08-31 Mainline default full channel body (default-full-channel-body)

## Overview

The body committed on the mainline changed from an empty table to a **full body**:
`internal/generated/plugins/zz_register.go` registers all 5 first-party channel plugins
(telegram, dingtalk, discord, feishu, qq). `just run`, the embedded UI binary, Docker,
and `just build-split` now include all adapters out of the box; Settings → Channels no
longer shows the `This generation has no ears` empty state.

Background: the user saw the empty-state wording in Settings; the confirmed decision was
"make the full version the development default."

## Scope of changes

- `internal/generated/plugins/zz_register.go` — empty table → full registration of 5 channels.
  The file header changed from "Code generated / DO NOT EDIT" to a hand-written body declaration;
  pack's `-overlay` still replaces the same path at build time, so narrow-generation assembly is unaffected.
- `go.mod` / `go.sum` — the root module now requires and replaces the 5 standalone plugin modules
  (`example.com/vivy/plugins/<name>` => `./plugins/<name>`), and merges the plugins' third-party
  dependency closures (telego, dingtalk-stream-sdk, discordgo, oapi-sdk-go, botgo, etc.).
- `plugins/discord/go.mod|go.sum`, `plugins/qq/go.mod|go.sum` — reran `go mod tidy`. See notes for
  the root cause: the plugin `replace agent-vivy => ../..` pulls the root module's entire dependency
  graph into the plugin MVS. After the full body raised shared dependencies (x/net → v0.50, taking
  x/crypto → v0.48 with it), the plugins' old pins no longer matched; `go list` (readonly) reported
  "updates to go.mod needed", and verify failed as a result.
- `sdk/internal/pack.go` — made `-modfile` merging idempotent: plugin modules already required+replaced
  by root go.mod skip the append (a duplicate replace causes a conflicting-replacement build error),
  while third-party closures continue to merge normally. Added `parseReplaceTargets` parsing (single-line + block forms).
- `sdk/internal/pack_test.go` — added idempotence tests (when root carries telegram, the merged result
  contains exactly one require + one replace and is parseable by go-command) and a
  `parseReplaceTargets` unit test; `TestOverlayGoModMergesPluginRequires` now uses a synthetic module
  not carried by root (the merge path is still genuinely covered); the MVS drift test's scratch check
  absolutizes local replace targets; removed the stale assertion that the "species go.mod must not
  contain telego" (legal under the full body; the actual invariant is that pack does not modify live
  files, and byte-level assertions already cover that).
- `docs/architecture/VIVY-CHANNEL-PACK.md` — restated the contract wording "the default committed body is empty"
  as "the mainline commit has the full body; pack narrows it at build time."
- `docs/TODO.md` §0.1 — recorded the `TFLAKE-CRON` flake (an existing timing-sensitive test unrelated to this change).

## Explicitly not done

- hello-fs (the tool-world demo) is **not** included in the full body—"full" refers only to channels.
- The UI empty-state wording `This generation has no ears` is retained: a narrow generation produced by pack (such as a pure-tool combination) still displays it correctly.
- Named packaging recipes (`generations.yaml` / `just pack <name>`) were not implemented; they will be handled separately.
- The Studio lifecycle flow was not changed; pack → eval → release → install semantics are unchanged.

## Commit

One Concern was committed on `feat/default-full-channels` and fast-forwarded back to `main`.
Not pushed (explicit user authorization required).
