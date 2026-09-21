# Verification — channel gate-0

All work ran in the worktree `../agent-vivy-channel-gate0`
(`feat/channel-gate0`, base = `feat/channel-hardening` 5768f46). The root
tree stayed untouched (it hosts the tier1 lane's in-flight approval work).
Fresh-worktree scratch: `ui/dist/.keep` placeholder for the go:embed
pattern and `pnpm install --frozen-lockfile` + `pnpm build` in `ui/` —
both gitignored, needed before any test that compiles `internal/app` or
runs the UI conformance suites.

## Per-commit scoped checks

- Commit 1 (discovery):
  - `gofmt -w` on all touched files.
  - `go build ./internal/... ./sdk/port/... ./plugins/...` (clean after
    the ui/dist placeholder).
  - `go test ./internal/channelhost ./internal/app -count=1` — ok
    (includes the new `TestDiscoverResolvesCapabilitySourceThroughWrappers`
    and `TestBindChannelsExposesAdapterCapabilities`).
  - `go test ./sdk/internal/assembly -run TestChannelProvidersAdvertiseExactlyTheirAdapterSurface -count=1`
    — ok (five ears advertise exactly `Capabilities{}` through the full
    bind chain).
- Commit 2 (ceilings):
  - Plugin tests from inside each module (they are separate Go modules;
    `go test ./plugins/<x>/...` from the root does not resolve):
    `cd plugins/<x> && go test ./... -count=1` for
    dingtalk/discord/feishu/qq/telegram — all ok.
  - Assembly pin extended with Definition ceilings + bound
    `RunesLimiter` passthrough — ok.
- Commit 3 (splitter):
  - `go test ./internal/channelhost -run TestSplitRunes -count=1 -v` —
    both `TestSplitRunes` (untouched) and the new
    `TestSplitRunesPreservesFencedCode` pass.
  - Full `go test ./internal/channelhost -count=1` — ok.

Two test literals were corrected during development: the hand-walked
expectations for the reopen cases under-counted the chunk count by one
(the splitter is right; the walkthrough missed a round). Fixed before
commit.

## P9 conformance refresh (per code commit — command sequence)

For every commit that touched a hashed root (commits 1 and 2: `internal/`
+ all five plugin roots; commit 3: `internal/` only):

1. Compute the new digest with the current declared value:

   ```
   go run ./sdk/internal/cmd/source-hash internal <current-internalSHA>
   go run ./sdk/internal/cmd/source-hash plugins/<x> <current-plugin-digest>
   ```

   Digests rotated: internal `6d88cd29…` -> `615826f0…` -> `45fcf5c8…`;
   dingtalk `023bf2d6…` -> `4886d9c9…` -> `a2ccc0d6…`; discord
   `82b537cc…` -> `b48dc145…` -> `4715b2c2…`; feishu `0a5b062c…` ->
   `d3f2b095…` -> `0f6b498d…`; qq `e422eccc…` -> `0a38f2e8…` ->
   `320e59a0…`; telegram `c3b73462…` -> `a4cc601e…` -> `61436c68…`.

2. Write the printed digest into the self-describing pins of that plugin:
   `plugins/<x>/vivy-module.yaml` (`source.sha256`) and
   `plugins/<x>/module_v1.go` (`Source.SHA256`). Plain hex-string sed
   replacement; CRLF is a non-issue (`sourcehash.Tree` normalizes to LF).

3. Verify the fixed point — re-run the command with the NEW digest as the
   declared value; it must print itself. All five plugin roots and all
   three internal rotations checked: fixed point OK.

4. Update the evidence files:
   `sdk/internal/conformance/reproduction_test.go` (`internalSHA` const +
   the channel entries' `SourceSHA256`) and
   `sdk/internal/assembly/conformance_results.json` (`sourceSha256`;
   the internal digest appears in all 5 internal-rooted entries).

5. Run the producer gate:
   `go test ./sdk/internal/conformance -run TestCheckedInProviderConformance -count=1`
   — ok after each rotation (305s first run failed only on the UI fixture
   suites because the fresh worktree had no `ui/node_modules`;
   `pnpm install --frozen-lockfile && pnpm build` fixed it; subsequent
   runs 144s / 153s / 151s, all ok).

`sdk/port/channel/channel.go` is inside no hashed SourceRoot, so the seam
addition required no digest rotation (verified by listing the SourceRoots
in `releaseSuiteCases()`).

## Full gate

- `just ci` — run at batch end from the worktree; result recorded below
  in the final status and in the merge commit message context.

## Not run, and why

- No real-path browser smoke: the batch ships no user-visible behavior
  change (capability bits stay all-zero on this base; splitting only
  differs for replies containing an unterminated fence at the cut, which
  the five current ears cannot produce through text-only v1 sends).
  `channel/inspect` output changes only when an adapter implements an
  optional interface, which first happens on the tier1 rebase.
- Root-tree checks were not repeated: the batch never touched the root
  tree or the `feat/channel-tier1` branch.
