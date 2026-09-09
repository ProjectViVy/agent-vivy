# Comprehensive review — complete findings list

Severity: 🔴 blocker (0) / 🟠 should-fix (17, disposition in parentheses) / ⚪ note (representative excerpts; see each lane's original report for the complete set). Disposition: **fixed** = landed in the fix round; **boarded** = new line in `docs/TODO.md` §0.1; **not fixed** = recording is sufficient.

## L1 Contract compliance (PASS)

- 🟠 F1 §8 "error classification" slot has no SDK landing point and was not registered → **boarded** (CH-R-1)
- 🟠 F2 §9.3's "import picoclaw/.workspace prohibited" rule was not implemented in verify → **fixed** (bannedImportPrefixes + `.workspace` substring rule + bad-picoclaw-import fixture)
- 🟠 F3 §10's per-seam inspect list exists only in the C2 log and was not boarded → **boarded** (CH-R-5)
- ⚪ F4 §20's "pack empty list remains an empty body" is literally unexecutable (pack rejects zero --with); the invariant is actually maintained by zz_register=nil + zero SDKs in go.mod → not fixed
- ⚪ F5 §14.3's qq "channel text" was not delivered (source review confirmed that the SDK cannot resolve a group address; the deviation is recorded) → recommend writing it back into the contract; pending SDK support
- ⚪ F6 The §12 payload sketch differs from the implementation → recommend writing it back into the contract (an identifiers-only payload is the better design); CH-C1-N2 includes the review conclusion
- ⚪ F7 The C2 log's claim that it was "registered in §0.1" was false (the line did not exist) → **fixed** (added the CH-R-4 memo line, noting that startup-time fallback already existed)
- ⚪ F8 There is no user-facing documentation for the `channels:` envelope and settings overlay (config.example.yaml/README have no section) → not fixed (recorded in findings; follow-up documentation slice)
- ⚪ F9 The §13 "post-slice read-only RPC" and C5's update RPC are compliant (writing the settings.yaml file and taking effect after restart is precisely the opposite of what §13 prohibits); recorded as a note
- ⚪ F10 = the closed-loop aspect of F7 (`partitionChannels` fallback exists)

## L2 Kernel correctness + security (PASS)

- 🟠 1 deliverCompleted/OnRunEvent nil-channel panic shape (unreachable in the assembly order; comment and code disagreed) → **fixed** (two guards + TestDeliverCompletedDropsUnregisteredChannel)
- ⚪ 2 Theoretical race where the terminal state precedes target registration (three statements vs. one model round trip; it can only drop, not crash; tracked as CH-C3-N1) → not fixed
- ⚪ 3 Session-ID NUL alias merging (platform IDs cannot contain NUL; 64-bit truncation is sufficient) → not fixed
- ⚪ 4 `allow_from` uses exact whole-string matching with no bypass; direction is always fail-closed (case/whitespace only cause false rejection) → not fixed
- ⚪ 5 `*_env` audit: nesting/aliases/merge keys/RPC injection cannot expand privileges; the actual state of CH-C6-N2 is slightly more nuanced than the board view (Windows is case-insensitive) → keep OPEN
- ⚪ 6 All discard-class early exits return nil + structured logs (identifiers only); policy discards leave no Journal trace (a product gap) → not fixed
- ⚪ 7 Exactly-once delivery holds; loss during the shutdown window is bounded (CH-C3-N1) + the CH-C4-N1 runes limit is not enforced → keep OPEN
- ⚪ 8 StartAll/StopAll/Inspect are clean; two StartAll calls would register twice (the app calls it only once) → not fixed
- ⚪ 9 Overlay merge order is sound (ghost names are dropped first, `partitionChannels` provides fallback, opaque nodes are not modified)
- ⚪ 10 All RPC error paths + zero key return flow passed review; unknown fields in channel/update are silently ignored (there are no secret fields to smuggle) → not fixed
- ⚪ 11 The Provenance nil path is byte-for-byte equivalent to before C1; forgery is an internal-kernel-caller issue (tracked as CH-C1-N4)
- ⚪ 12 Provenance columns are equivalent across both engines; the frozen v14 fixture genuinely drives the in-place upgrade; the DSN-gating gap is CH-C1-N5
- ⚪ 13 All four vendor log-risk mitigations are in place (telego redaction/botgo silence + assertion/discordgo pinned LogLevel/feishu no-token logging)

## L3 Adapter cross-cutting review (PASS; every cell in the 20-row × 5-plugin consistency matrix is ✓ or has a recorded deviation)

- 🟠 F1 DingTalk becomes deaf after a network-level silent disconnect (the SDK's Start returns immediately while conn is alive; only a graceful disconnect frame triggers redial; the loopback test does not cover a dead link) → comment corrected + **boarded** (CH-C6-N3); behavioral fix deferred
- 🟠 F2 DingTalk/Feishu restart-after-stop latches were not reset (Feishu could potentially hang in Start) → **fixed** (Start reset + TestStartAfterStopStartsFresh ×2)
- ⚪ F3 Telegram's late-callback barrier is join-like (one callback may be in flight after Stop ctx is canceled; Host safely drops it) → not fixed
- ⚪ F4 SDK logger posture is asymmetric (QQ silent/DingTalk fully blind/Feishu default/telego redacted/discordgo pinned) → define together with the follow-up ChannelEnv logging surface (CH-C6-N1)
- ⚪ F5 The QQ sender shape `qq:user_<openid>` is the sole deviation from bare `<platform>:<id>` (documented; operations must know)
- ⚪ F6 DingTalk webhooks / QQ chats runtime maps have no upper bound (small strings, same shape as picoclaw) → not fixed
- ⚪ F7 Verified no-action items: Feishu encrypt_key is lazy for WS, Discord does not redeliver so deduplication is unnecessary, and Telegram 4096 is the API limit
- The five SDK claims were all confirmed from source (telego redaction logger.go:98-103; dingtalk WithAutoReconnect option.go:15 + reconnect loop Background ctx; lark v3.11 Start returns vs. v3.9.4 select{}; botgo identify frame at INFO level; discordgo Identify only LogDebug)

## L4 SDK/pack (PASS)

- 🟠 1 The Listen ban could be bypassed through method calls/ListenPacket/tls.Listen (empirical probe) → **fixed** (ban any receiver method name + tls.Listen + bad-channel-listen2 fixture; the conservative false-positive direction is commented)
- 🟠 2 pack silently discarded plugin replace/exclude directives for non-agent-vivy modules → **fixed** (explicit error + tests for block, single-line, and valid forms)
- 🟠 3 pack of two standalone modules had no test; repeated --with values were not deduplicated → **fixed** (real build in TestPackTwoStandaloneModules + deduplication by resolved directory + test)
- 🟠 4 The C2 log's "registered in §0.1" claim was false → **fixed** (CH-R-4)
- ⚪ 5 verify rule↔fixture completeness table: 14 negative fixtures map one-to-one; about 18 rules have no fixture (mostly existing apiVersion/semver lines); the §9.3 picoclaw line was previously unimplemented → **fixed**
- ⚪ 6 The ABI family is coherent; the most likely C8+ signature breaks are ordered: added ChannelEnv methods > MediaStore missing Get > two WebhookHandler method sets > the always-true assertion trap in empty interfaces TaskLifecycle/PipeServer (commented, not guarded) > InboundMessage missing display name (additive and safe)
- ⚪ 7 capabilities.go confuses the category of the MediaStore assertion for plugins (harmless; nobody implements it) → not fixed
- ⚪ 8 pack parsing edge cases (end-of-line comments/`require(` without a space/block comments) → loud downstream failure; not fixed
- ⚪ 9 Nine "C4 pins the ABI" wordings in channel.go are stale (all five adapters have optional capabilities implemented as plain text) → cosmetic; defer
- The 13-line failure-mode table is fully recorded (in the original report); a test chain proves that the live-tree bytes are unchanged (zz_register/go.mod/go.sum together)

## L5 UI (PASS; all 8 deletion-list items have zero dangling references)

- 🟠 1 inspect failure still rendered the `This generation has no ears` empty state (the error was only 12px toolbar text) → **fixed** (error panel + loadFailed i18n)
- 🟠 2 The wizard's `enter platform credentials` wording overpromised (having no token input is intentional by design) → **fixed** (changed to `prepare credential environment variables` + set env outside the UI + restart guidance, zh/en aligned)
- 🟠 3 The tutorial text was stale and zh-only (it described the deleted credential form) → **fixed** (steps rewritten to match the real flow)
- 🟠 4 The discord guild_id placeholder contained the audit-prohibited wording `leaving it blank means no limit` (a metadata field) → **fixed** (changed to `blank = process all servers`)
- ⚪ 5 The sandbox domain placeholder in zh.ts uses the same phrase (not part of the channel surface; the semantics are correct as-is) → not fixed
- ⚪ 6 Multiple functions exported by channel-schema are consumed only by tests (the reason "C6/C7 reuse" is no longer valid) → cleanup candidate
- ⚪ 7 Dead i18n keys (channels.channels, the diva.channels block, fake 1/3-ready statistics) → cleanup candidate
- ⚪ 8 Remounting does not automatically refetch (a manual refresh button exists; there is no cache between refreshes) → recording is sufficient
- ⚪ 9 Test gaps: the value of the fail-closed wording is not pinned (only the key name is pinned), there are no component-level tests, and api.test asserts only method names → follow-up needed
- All 8 deletion-ledger items have zero dangling references; data truth, key discipline, i18n alignment, and a11y all pass

## L6 Documentation board (PASS; the item-by-item verification table for all 9 logs passes)

- 🟠 1 Ghost branch `feat/channel-c7a` (12a2a70 actually landed on the c6 line; four inaccurate wordings in the CH-C7a banner/C7b two logs/TODO §0.2.7) → **fixed**
- 🟠 2 `ChannelEnv.Settings()` was not written back into CHANNEL-PACK §9.3 and PLUGIN-SPEC §4 (implementing from the docs would not compile) → **fixed** (added the fifth method in both places)
- 🟠 3 The stale `UI-CHANNELS-BE` line remained OPEN (CH-C5 had already claimed the delivery) → **fixed** (set to DONE)
- 🟠 4 The V0 architecture document around line~300 missed the `CN-01..16` update → **fixed** (CN-17)
- ⚪ 5/6/7/8/9/10: C3 test-name spelling, C5 file-count convention, stale CH-C1-N3/N4 routing (N3 updated during the fix round; N4 remains), and the C7a SDK argument tree being unverifiable outside the tree (disclosed) → recording is sufficient
- Left scan: zero scratch/binary/routeTree contamination across 9 commits; zero TODO/FIXME in new Go/UI code; go.mod/go.sum byte-for-byte unchanged throughout

## TEST-3 (newly created during the review)

- 🟠 Two `just ui-e2e` cases failed (the full runtime flow and welcome-wizard) — rerunning the 82ecf14 baseline failed identically → **boarded** (§0.1 TEST-3: e2e baseline is broken; unrelated to the channel EPIC)
