# Acceptance — channel gate-0

How a human can tell each fix landed. The batch is infrastructure: it has
no wizard or screen of its own, so the checks are inspect-output,
long-reply behavior, and code reading.

## 1. Capability discovery is honest

- Start the split dev pair (`just dev`) and open
  `http://127.0.0.1:3015`; open the channels settings surface and trigger
  a channel inspect (`channel/inspect`). Every compiled ear reports
  `capabilities` with all bits false on this base — the five text-only
  adapters implement nothing optional yet, and honest discovery must say
  exactly that.
- The proof that the pipe now works is in tests, not in the UI:
  `TestDiscoverResolvesCapabilitySourceThroughWrappers` (adapter
  capability visible through two wrappers),
  `TestBindChannelsExposesAdapterCapabilities` (full bind chain), and
  `TestChannelProvidersAdvertiseExactlyTheirAdapterSurface` (the five
  compiled ears advertise exactly their adapter surface — zero today).
  After the tier1 rebase (CH-R-1 adds `Health` to all five adapters),
  the same inspect starts showing `health: true` per ear — that flip is
  the visible sign the gate opened.

## 2. Long replies no longer reach the four ears unbounded

- Configure any of dingtalk / discord / feishu / qq, make the assistant
  produce a reply longer than the ear's ceiling (e.g. ask for a 3000-word
  answer on Discord). The reply arrives as several messages instead of
  one API rejection. Ceilings: discord 2000, dingtalk 5000, feishu 37500,
  qq 2000, telegram 4096 runes.
- Each ceiling's platform source is readable next to the code that
  declares it (`plugins/<x>/module_v1.go`, Definition comment) and in the
  contract §1 Decision Record — including the honest "no official number
  published" note for qq.

## 3. Code blocks survive splitting

- Ask for a long answer dominated by one big fenced code block (longer
  than the ear's ceiling). Every delivered message renders as valid
  markdown: no message ends with an unclosed fence, continued code
  arrives in a message that reopens the same fence with the same
  language tag. Replies without fences split exactly as before.

## Regression guards

- `just ci` green, including the P9 conformance gate — the checked-in
  evidence matches the executed suites for every commit.
- `TestSplitRunes` is byte-identical to before this batch; the no-fence
  splitting path cannot drift without it failing.
