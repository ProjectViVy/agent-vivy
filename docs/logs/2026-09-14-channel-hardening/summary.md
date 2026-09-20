# 2026-09-14 — Channel hardening batch (CH-C3-N1, CH-C1-N2, CH-C1-N4, CH-C6-N3)

## What changed

The batch closes the four open channel items on `docs/TODO.md` §0.1 in one
lane, branch `feat/channel-hardening`:

1. **Durable outbound reply intents (CH-C3-N1 core).** New
   `storage.ChannelDeliveryStore` (`channel_deliveries` table: sqlite
   `migration023`, postgres schema v21). The ChannelHost records an `armed`
   intent when an inbound turn opens its run, flips it to `pending` before
   the delivery goroutine spawns, deletes it on success, parks it `failed`
   after bounded attempts (the attempt count persists across restarts), and
   settles it on failed/cancelled terminals. `Host.StartAll` reconciles open
   rows against the journal after runtime recovery: armed+completed →
   deliver, armed+ended → settle, pending → redeliver. Delivery semantics
   are **at-least-once** (documented in contract §12). `StopAll` now stops
   accepting delivery spawns and waits — bounded by the shutdown deadline —
   for in-flight Sends before stopping adapters; intents that do not drain
   stay pending for the next start. Conformance cases CN-27 (delivery
   intents) and CN-28 (retention) bring the suite to 28 cases.

2. **`chanin_*` retention (CH-C3-N1 remainder).** New
   `storage.ChannelMaintenanceStore.PruneChannelInboundEvents` on both
   backends; the app prunes once per process start with the constant
   `storage.ChannelInboundRetention` (30 days, matching the
   `FileVersionRetention` no-config precedent). Non-chanin events are never
   touched (sqlite `GLOB`, postgres escaped `LIKE`).

3. **Source vocabulary validation (CH-C1-N4).** Architect ruling
   (2026-09-14): the closed vocabulary is `ui | channel | headless`.
   `domain.SourceUI/SourceChannel/SourceHeadless` + `ValidMessageSource`;
   `RunWithOptions` rejects any other value with a typed error before
   anything persists (empty Source with a non-nil Provenance stays
   rejected; the empty value remains a legacy/in-process reading handled by
   `EffectiveSource`). Producers now use the constants; per-platform names
   stay in the `Channel` field.

4. **Contract §12 rewrite (CH-C1-N2).** `VIVY-CHANNEL-PACK.md` §12 now
   carries the implemented identifiers-only journal payload
   `{channel, chat_id, sender, message_id, session_id}`, the
   journal-after-ensure sketch order, the source vocabulary ruling, and
   net-new outbound-durability / retention text; a dated ruling block and a
   §1 Decision Record row record both architect rulings.

5. **Dingtalk silent-link deafness (CH-C6-N3 residual).** The board row was
   stale: PLG-P1 (2026-09-10) had already replaced the SDK `StreamClient`
   with the in-house `governedStreamClient` (former option (b), conn
   cleared on any readLoop exit). The residual gap — a truly silent link
   (no FIN/RST) blocked `ReadMessage` forever — is closed in-house: the
   client pings the socket on a tick and bounds every read with a deadline
   (pong/data frames move it forward; knobs are captured at construction so
   no running goroutine reads the package vars). A dead link now trips the
   deadline and the existing supervise tick redials. The module's
   self-describing source hash was recomputed for `vivy-module.yaml` and
   `module_v1.go`. No upstream patch is needed.

## Scope

Kernel + storage + channelhost + dingtalk plugin + channel contract doc.
UI, RPC surfaces, settings, and the packed-candidate flows are unchanged
(the only app-side wiring is passing `Deliveries: backend` and the startup
prune call).

## Explicitly not done

- CH-C8 (`vivy channel --name` child process), CH-C9 (A2A/NeuroLink), WeCom
  (`CH-C`), CH-R-1 / CH-R-4 memos — untouched, still start-by-naming only.
- Real-Postgres verification of migration v21 (CH-C1-N5 environment debt,
  shared with STORAGE-ATTACH-PG-TEST).
- Live-bot platform smoke for any of the five ears (no bot tokens in this
  environment); the composed-organism boot smoke is recorded in
  `verification.md`.
- Operator-facing redelivery control for `failed` intents (the row is
  storage-visible; a re-delivery RPC/CLI would be a separate product
  decision).
