# Acceptance — 2026-08-31 default-full-channel-body

## How to verify that it took effect

1. From any new checkout of the repository (or the existing root), run `just run` directly, then open
   `http://127.0.0.1:3015` (split Vite) or `http://127.0.0.1:8787` (embedded).
2. In the left navigation, `Settings` → `Channels`: you should see five channel cards for **DingTalk, Discord, Feishu, QQ, and Telegram**, rather than the `This generation has no ears` empty state.
3. Each card displays `Disabled · no config envelope`—this is expected: the channel is compiled in but has no configuration envelope yet. Click `Enable`, configure `token_env` and `allow_from`, and restart the process before the adapter actually starts.
4. The semantics of `channel/inspect` are unchanged: an unconfigured channel appears as compiled in but not started, with the corresponding log `channelhost: channel compiled-in but not configured; not started`.

## Regression perspective

- To get a narrow generation with **no** particular channel: `vivy-sdk pack --with <plugins-to-keep>`, producing `dist/<gen>/vivy.exe` + `generation.json`; that binary's Settings page will show the empty state again (if no channel is included).
- `docker-up` / `just build-split` and the everyday `just run` now produce the same full generation; there is no longer a distinction of "only packaging gives you adapters."
