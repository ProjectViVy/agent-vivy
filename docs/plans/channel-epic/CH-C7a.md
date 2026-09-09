# CH-C7a — `plugins/feishu` Private-Chat Text over WS

## 1. Identity

| | |
|---|---|
| ID | CH-C7a |
| Stage | F |
| Person-days | 2 |
| Milestone | M-CH3 |
| Dependency | CH-C3; **CH-C4 should be merged** (ABI template) |
| Branch | `feat/channel-c7a` |
| Contract | §14.3 feishu; implementation is WS, public webhook from the document is out of scope |

## 2. Goal

Independent `plugins/feishu`. Feishu/Lark private-chat text, with an outbound SDK WS. Put the `is_lark` domain switch in settings. 64-bit only; 32-bit must fail to compile with a clear error. The default EXE has no lark SDK.

## 3. Current State

The contract explicitly excludes a 32-bit stub, emojis, and public webhook mode.

**Note:** Before formally writing the Feishu adapter, first read picoclaw—it is the **most complete** Go sample among the channel implementations. Read-only rewrite; imports are prohibited. Reference: `.workspace/picoclaw/pkg/channels/feishu` or `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\feishu`. The implementation is WS; do not turn it into the public webhook described in the document. See `00-standing-orders.md`.

## 4. Target Structure

Copy the C4 directory. Put `encrypt_key` in plugin settings; the Host does not decode it. grants: `channel.poll` + `secret.read`. Credential env keys: `app_id` / `app_secret`.

## 5. File Inventory

**Create** `plugins/feishu/**`. Do not add lark to a species go.mod; do not make the Host know the encryption algorithm.

## 6. Steps

1. Independent module; rewrite the WS event loop.
2. `is_lark` settings.
3. verify + pack --with feishu.
4. Note 64-bit in documentation/tests.
5. `just ci` default path.
6. Log to `docs/logs/YYYY-MM-DD-channel-c7a/`.

## 7. Acceptance

- Candidate private-chat text works; the default has no lark.
- Empty allow_from is rejected.
- No public webhook implementation.

## 8. Prohibitions

- 32-bit compatibility layer.
- `channel.webhook` grant.
- Emojis / media (later slice).

## 9. Risks and Rollback

- Feishu SDK bloat: an independent go.mod is mandatory and is a hard acceptance criterion.
- Rollback: do not name it in the recipe.

## 10. Handoff

[CH-C7b.md](CH-C7b.md).

> **DONE 2026-08-30** — commit 12a2a70 landed on the feat/channel-c6 line (not split into a separate branch). SDK selection template: Feishu WS must use oapi-sdk-go/v3 ≥ v3.11.0 (older lifecycle is broken); two `*_env` secrets + `encrypt_key`/`is_lark` go through plugin settings. Filing: `docs/logs/2026-08-30-channel-c7a/`.
