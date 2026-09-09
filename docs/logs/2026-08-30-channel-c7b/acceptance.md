# CH-C7b — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: the official QQ bot ear—only the sanctioned route

A bot registered on the QQ Open Platform receives direct-message text over a WS long
connection → journals it → Run → replies through the official v2 API's passive-reply
window. No personal-account protocol reverse engineering, no OneBot/NapCat sidecar
process, and no second body.

## Human-verifiable points

1. **The gate is green**: `just ci` exits 0; botgo does not appear in the
   default EXE dependency graph.
2. **Identity is clean**: package comments, settings comments, and README all state
   non-personal-account / non-OneBot / non-NapCat; the plugin connects only to the
   official QQ Gateway and official v2 API.
3. **Packaging is enough**: `vivy-sdk verify plugins/qq` → ok;
   `pack --with qq` → candidate EXE (inspect lists qq); the product tree's
   `go.mod` is byte-for-byte unchanged. The C5 Settings page automatically
   shows a QQ card.
4. **Real-client loopback** (inside CI, no real network): the real botgo OpenAPI client
   hits a local stub; the passive-reply path, auth headers,
   `msg_id`/`msg_seq` contract, and surfaced error codes are all asserted.
5. **Fail-closed discipline is unchanged**: empty allow_from refuses Start; an unset env
   fails Start with a visible reason; after restart, a missing passive-window msg_id
   produces a clear error instead of sending to the wrong place.
6. **Key discipline**: botgo's default logger writes access tokens and message content
   at INFO—the plugin globally installs a quiet logger (pinned by tests); the token is
   cached only in memory.

## Explicitly not part of this slice's acceptance

- Group-chat @ replies → botgo v0.2.1 cannot decode `group_openid` (verified in
  source); wait for the official SDK to fill the gap before proposing it.
- Voice / large files / Guild → prohibited by the contract.
- Real QQ Open Platform send/receive → pre-release human acceptance (requires bot
  credentials; rollback = recipe does not name qq).

## Rollback

Omit qq from the recipe and it disappears; revert this branch and this ear is gone.
