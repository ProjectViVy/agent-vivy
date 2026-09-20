# CH-P0-2 backend owner

Baseline: CH-P0-1 at `22acb6982d1d4abce948acf08a8575fb98d5669f`.

CH-P0-2 makes `vivy/channel-host` the real owner of Channel backend behavior.
The application and RPC dispatcher no longer import or hold the concrete
Channel Host; generated Assembly, app composition, runtime delivery, and
management dispatch meet through the CH-P0-1 contracts.

## Delivered

- `internal/modules/channel` owns construction, Provider binding, Grant-limited
  Host facades, private `channelhost.Host` composition, inspection, lifecycle,
  and the three existing management methods.
- Generated `RuntimeAssembly` exposes exactly one `ChannelFactory` for the
  canonical Host and no longer constructs the placeholder Host owner through
  the generic lifecycle list.
- `internal/app` adapts effective configuration and the shared settings
  document into focused contract values, constructs the owner even under
  `WithoutEars`, attaches it as the existing run hook and delivery authority,
  and owns contract-level startup, rollback, and shutdown ordering.
- `internal/rpc` accepts generic validated contributions. Channel capability
  names and handlers appear only when the selected owner contributes them.
- Existing `channel/inspect`, `channel/get`, and `channel/update` wire shapes,
  validation codes, frozen/read-only behavior, and successful-update
  notification remain covered by Module-owned tests.
- SDK Port conformance now exercises the Module-owned Provider binding rather
  than the removed app-owned boundary.

## Boundary after this slice

| Layer | Channel knowledge retained |
|---|---|
| generated Assembly | `channelcontract.Factory`, selected Providers and Grants |
| app | contract dependencies, selection, settings adapter, lifecycle orchestration |
| RPC core | generic `rpccontract.Contribution` validation and dispatch only |
| canonical Module | binding, grants, Host engine, management DTOs/handlers, inspection |
| private Host | stable session, inbound, delivery, and Provider runtime pipeline |

## Explicit exclusions

- No conditional concrete Module import, reduced Recipe, or backend physical
  omission claim (CH-P0-3).
- No Channel UI relocation, settings-section contribution, visibility rule,
  or UI asset omission (CH-P0-4).
- No release selection matrix, packaged omission proof, browser/network
  acceptance matrix, or rollback release record (CH-P0-5).

The default Generation remains linked to the canonical Module through the
transitional defaults constructor and still selects DingTalk, Discord, Feishu,
QQ, and Telegram.
