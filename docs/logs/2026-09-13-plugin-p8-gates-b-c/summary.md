# PLG-P8 Gates B/C summary

Date: 2026-09-13

Status: **PASSED — PLG-P8 COMPLETE**

Gate A was merged in PR #24. This continuation closes the owner-authorized
bounded SCX slice without inventing the unrecovered original SCX stage IDs.

Gate B now has executable integration evidence for:

- versioned, scoped Context Source candidates and exact-version resource
  resolution through ContextHost;
- deterministic required/optional selection, byte budgets, expiry, redaction,
  provenance, Treatment-bound immutable Context View identity, and fail-closed
  required-source behavior;
- Runtime projection during initial run preparation, with the selected View
  committed in `model.request` v2 and recovered across approval/question
  resume;
- receipt-aware committed terminal-event delivery through ObserverHost, with
  sealed event subscriptions and allowed-field projection, structural secret
  redaction, retry, cursor recovery, and one logical update after ambiguous
  acknowledgement; and
- a real bounded `scx/reference-fixtures@0.1.0` provider that traverses Context
  Source -> ContextHost -> Runtime -> Journal -> ObserverHost.

Gate C now has executable deterministic rebuild, module-removal, packed health
probe, Studio install/rollback, and Journal-preservation evidence. The SCX
candidate is Generation
`40c2e3c2302a23b277bab5db159d2bbd59d8af60477ce6f568ce81c5d5d3e5a5`.
Default and minimal removal controls are respectively
`b5f5b71eb388974cb8b0fef58b077046ad1ae5790fa58c29b5f8e4ae8104469d`
and `647278455905b6c1bc83abdbf8b48a546d1eacb7fcd28bd7a007d2051db714ad`.

This closes PLG-P8 only. The selected P9 conformance matrix used by Gate C is
complete; the broader PLG-P9 release program remains unscheduled and unclaimed.
Refreshing context inside one Eino tool loop, rich-media resource projection,
external live SCX systems, and embodied/device integrations are also outside
this bounded slice.
