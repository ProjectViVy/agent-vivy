# Acceptance — CH-C1-N3 provenance

## How to verify manually

1. After binding a channel (such as the Telegram plugin), send Vivy a message from
   that platform.
2. Open the same session in the web UI (`http://127.0.0.1:3015`): a small
   provenance marker should appear above the user message bubble (such as
   `telegram · chat-1`).
3. Messages sent directly in the UI have no provenance marker (rendering remains
   pixel-for-pixel identical to before the change).
4. At the API level, channel user messages in the `session/messages` /
   `session/get` responses include
   `"provenance":{"source":"channel","channel":"…","chat_id":"…",
   "channel_message_id":"…"}`; UI messages have no `provenance` key.
5. Historical data (empty Source rows written before the change) is read as ui,
   with no marker and no error.

## Regression surface

- Existing message-list and attachment rendering (data URL images) is unchanged;
  the ui-e2e runtime.spec is all green.
- The RPC contract only adds a field (`omitempty`), so old clients are unaffected.
