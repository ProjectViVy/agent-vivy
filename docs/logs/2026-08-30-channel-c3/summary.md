# CH-C3 — ChannelHost + fake-plugin TCK (summary)

Date: 2026-08-30. Branch `feat/channel-c3` (cut from `feat/channel-c2`
edf24c8; sequential slices reused the same worktree, with an independent branch per
slice).
PLAN: `docs/plans/channel-epic/CH-C3.md`. Contract:
`VIVY-CHANNEL-PACK.md` §7/§8/§12.

## What changed

The world entry exists for the first time: a fake channel sends text → `channel.inbound`
is journaled → `Message(source=channel)` → `Service.Run` → the terminal result
is sent back through the adapter's `Send`. The default body remains
`Register() = nil`, with no ear and zero real protocol.

- **New package `internal/channelhost/`** (zero eino and zero internal/runtime
  imports—covered automatically by importlint and confirmed by a manual grep):
   - `host.go`: `StartAll`/`StopAll`/`Started`. Fail-closed:
     unconfigured = included but not started; `enabled: false` = not started;
     **empty `allow_from` refuses Start and records an error**; a Start failure
     on one ear only skips that ear and does not kill the organism.
   - `session.go`: session mapping (decision: **deterministically derived ID**
     `sess_ch_<sha256(channel\0chat\0topic)[:16]>`, no new storage table, stable
     across restarts, and naturally separate from UI Sessions; title
     `channel/<name>/<chat>`). A concurrent first-message session-creation race
     converges by rereading.
   - `dispatch.go`: PublishInbound pipeline = envelope parsing (missing
     channel/chat_id/sender/message_id is dropped) → exact allow_from match (sender not
     on the list = dropped and audited, not an error) → EnsureSession →
     **journal `channel.inbound`** (decision: each message gets an independent
     pseudo-run scope `chanin_<16hex>`, payload fields align with the C1 schema,
     `run_id` omitted, and no terminal state is ever written) → extract text
     parts → injected `Run` function (with `domain.Provenance`) → track
     terminal state. Terminal delivery: `run.completed` → take the last
     non-tool assistant row for that run → `Send` (outside the runtime
     goroutine, `WithoutCancel` + 30s timeout); failed/cancelled only log in
     this slice and do not deliver.
   - `capabilities.go`: 11-bit `Capabilities` + `Discover`
     (type-assert each sdk/plugin capability interface; TaskLifecycle/PipeServer remain
     reserved without assertions).
   - `channelenv.go`: the Host's `plugin.ChannelEnv`—
     `Secret` is fail-closed (missing grant/empty value errors, value never
     enters logs), a 30s outbound `http.Client` (no Listen), the
     `PublishInbound` entry point, and a noop MediaStore.
   - `fake/`: test Channel (Start immediately publishes one hello; Send records
     in memory; no optional capability is implemented).
2. **Eight TCK cases** (real sqlite backend + injected Run stub + Journal-recording
   decorator): empty allow_from refuses Start and later inbound is dropped; an
   unlisted sender does not Run, create a session, or journal; an allowed sender →
   `chanin_` is journaled + all three `Source=channel` fields are stored
   + Run is called + the session ID is deterministic; terminal delivery happens exactly
   once and replaying the terminal state does not resend; failed is not delivered;
   unknown config names fail startup; Discover reports no capabilities for the empty
   fake.
3. **runtime seam (service.go; engine.go untouched)**:
   `RunOptions.Provenance *domain.Provenance` (new `domain.Provenance`);
   the nil path is byte-equivalent to C1's `Source:"ui"`, with no breakage for
   existing RPC/worker callers (pinned by tests).
4. **Capability interface completion (sdk/plugin/channel.go; names unchanged)**: seven
   of C2's nine reserved slots receive the minimal v1 method set
   (MediaStore/Typing/MessageEditor/Placeholder/MediaSender/WebhookHandler/
   StreamingCapable), and MessageDeleter/ReactionSender/ListenHandler/HealthChecker
   are added; TaskLifecycle/PipeServer retain zero methods (Stage H). Each comment is
   "Minimal v1 surface; the first real adapter (C4) pins the ABI."
5. **app assembly**: `partitionChannels`—a `channels.<name>` mismatch with
   the compiled set = **startup failure** (matching the tools.Resolve precedent); a
   channel seam that does not implement Channel also fails startup; the Host is built
   before NewService and attached to `RunHook` with structured compatibility;
   `StartAll` runs after `Recover` and before listen (failure aborts and
   closes the backend); `StopAll` is first in shutdown. `Register()=nil`
   behavior is exactly unchanged.

## Design decisions (including C1 carry-over closure)

- **CH-C1-N1 closed**: `channel.inbound` is journaled in a pseudo-run scope,
  with no journal-engine changes and no D-008 impact, and the contract is unchanged;
  `run_events` accumulates `chanin_*` events without run rows (lazy data;
  Recover/ListActiveRuns use the runs table), with retention recorded in §0.1.
- **CH-C2-N2 partially closed**: four new capability interfaces landed;
  `InboundMessage`/`OutboundMessage` run_id/task_id slots are **intentionally
  deferred** (this slice has no consumer under the pseudo-run design), the SDK comments
  now describe the deferral, and the TODO line is synchronized.
- The tension between the contract §12 sketch order (journal → Ensure) and the C1
  schema (session_id required in the payload) is the existing CH-C1-N2; this slice
  implements Ensure first (schema priority).

## Explicitly not done

- No real protocol, `vivy channel` subprocess (C8), or webhook/listen service
  surface; the Host does not know parse_mode/encrypt.
- Terminal delivery is tracked in memory: if the process exits between run.completed
  and Send, the reply is lost; `chanin_*` events are not cleaned up; `Secret`
  is not pinned to the envelope's `token_env` allowlist. All are recorded in
  §0.1 (CH-C3-N1/N2) and hardened when the real C4 adapter lands (including Send/Stop
  race safety).
- failed/cancelled deliver no copy to the adapter (a product decision for this slice;
  failure copy belongs to Face/later work).
- Not pushed.
