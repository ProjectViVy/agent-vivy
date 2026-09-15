# 2026-09-15 — channel gate-0 (capability structure gate)

Batch 0 of the picoclaw gap inventory: three structural fixes that make
every later optional channel capability (typing, health, media, edit)
possible. Shipped on `feat/channel-gate0` (worktree), to be merged into
`feat/channel-hardening` and carried by the same PR; the tier1 lane
rebases on top of it.

## What changed

1. **Capability discovery reaches the adapter.** `Discover` type-asserted
   the eleven optional interfaces against the bound channel, but the v1
   assembly chain (adapter -> `boundChannel` -> `providerChannel`)
   forwards only Start/Stop/Send, so every probe missed and inspect
   advertised zero for every ear. New `plugin.CapabilitySource` seam:
   `Discover` resolves the chain to the innermost target before
   asserting, `bindChannels` captures the provider's typed-nil probe at
   bind time, and each of the five `module_v1.go` providers exposes it
   next to `Construct` (one line in the copy template). Wrappers still
   declare no capabilities of their own; a missing or nil seam falls back
   to the wrapper, which honestly reports nothing.
2. **Outbound rune ceilings for the four remaining ears.** dingtalk 5000,
   discord 2000, feishu 37500, qq 2000 — each `Definition.MaxMessageRunes`
   with its platform source recorded in the module_v1.go comment and the
   contract Decision Record. Telegram keeps 4096; the Definition is now
   the single source and the adapter's shadowed duplicate method (with its
   stale vivy-plugin.json sync note) is deleted.
3. **Fence-aware `splitRunes`.** A cut that would land inside an open
   fenced code block now either extends to the block's real closing fence
   (when it fits) or closes the chunk and reopens the remainder with the
   original fence line (info string included), so every delivered chunk
   renders standalone. No-fence behavior is byte-identical to before;
   `TestSplitRunes` is untouched.

Contract: one new §1 Decision Record row. P9 conformance evidence
(`internalSHA` + five channel plugin digests, fixed-point verified) was
refreshed with each code commit, so every commit is individually green.

## What was explicitly not done

- No real optional capability is implemented: no Typing/Edit/Media
  behavior, and the Health **call path** (`providerChannel` forwarding +
  real probing) stays with tier1's CH-R-1 follow-up. Discovery is the
  only thing wired in this batch.
- No adapter behavior change: the only adapter-body edit is deleting
  telegram's shadowed duplicate `MaxMessageRunes`.
- No `deliver.go` state machine, RPC, or storage changes; no groups or
  webhooks; picoclaw is read-only reference (never imported).

## Base and expectations note

This batch is based on `feat/channel-hardening` (5768f46), where the five
adapters implement no optional interface, so the compiled ears advertise
the zero set — pinned by
`TestChannelProvidersAdvertiseExactlyTheirAdapterSurface`. The tier1
branch carries CH-R-1, which adds `HealthChecker` to all five adapters;
after that lane rebases, the pinned expectation flips to
`Capabilities{Health: true}` per ear, and `channel/inspect` starts
reporting it.
