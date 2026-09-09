# CH-C3 — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: the first ear moves—although it is fake

- The daily body **still has no ears**: default `vivy.exe` `Register()=nil`,
  no `channels:` entry → behavior is exactly as before C2, with zero real
  protocol HTTP.
- The change is that adding a "fake ear" to this generation and configuring it lets a
  message travel through the complete world-entry loop.

## Human-verifiable points

1. **The gate is green**: `just ci` exits 0 (including 8 channelhost TCK cases
   and the runtime Provenance regression).
2. **The loop holds (the TCK is the demo)**: fake channel `Start` →
   `PublishInbound` (sender on the allowlist) → the Journal gets a
   `channel.inbound` event (five payload fields, no token) → session
   `sess_ch_<hash>` is created automatically (title
   `channel/fake/chat-1`) → a user row with `Source=channel` and three
   provenance fields → Run starts → after the terminal state the fake adapter's
   `Send` receives the last assistant reply exactly once (replaying the
   terminal state does not resend it).
3. **Three fail-closed checks**:
   - Empty `allow_from` → the channel **refuses Start** (error log; no ledger
     entry, session, or Run);
   - sender not on the allowlist → dropped and audited (not an error), so the model
     never sees it;
   - `channels:` names a plugin absent from the body → **the entire startup
     fails** (matching the existing unknown-name semantics for `tools.enabled`).
4. **Local UI and channel sessions do not merge**: channel session IDs are derived by
   hashing (channel, chat_id, topic), while the random `sess_` ID created by the
   UI can never collide; the same chat maps to the same session after restart
   (deterministic derivation, no mapping table).
5. **An ear can be named and removed**: omitted from config = included but not started;
   `enabled: false` = visible in inspect but not started; removing the plugin from
   this generation = startup failure explaining that config points to a missing name.

## Explicitly not part of this slice's acceptance

- Real Telegram send/receive → CH-C4; DingTalk → CH-C6.
- Showing ear status in inspect/Settings → CH-C5.
- No lost replies after a crash (durable outbound queue), `chanin_*` event-retention
  policy, and pinning Secret to token_env → recorded in §0.1 and hardened from C4 onward.

## Rollback

Remove the app assembly (or revert this branch) and the ear disappears; default body
behavior is unchanged.
