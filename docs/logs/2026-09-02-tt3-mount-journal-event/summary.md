# TT-3: journal audit event for skill tool mounts — 2026-09-02

## Summary

`model.request` journals only the run's active tool baseline; when
`skill_view` mounted hidden tools mid-run (TT-1 two-tier tools), the mount
itself left no audit trail — it was only inferable from later tool events.
TT-3 adds the missing journal event.

- New domain event `tool.mounted` (`EventToolMounted`) with payload
  `{tool_name, tools}`: `tool_name` is the mounting tool (today
  `skill_view`), `tools` lists only the names newly mounted by that call.
- Emission is generic, not skill-specific: the tool adapter snapshots the
  run's `MountedTools` registry before invocation and journals the delta
  after a successful call. Any future tool that activates hidden tools is
  audited the same way.
- Flows through the existing governance-event sink (event budget
  accounting, persist + publish), mirroring `policy.evaluated`.
- Contract artifacts: `schemas/events/payloads/tool.mounted.json` (v1) and
  a `tool.mounted` row in the `run-event.schema.json` eventType enum.
  Go payload struct `payloadToolMounted` mirrors the schema field for
  field per the A3 convention.

Explicitly not done:

- No UI surface for the new event yet — the run event feed renders event
  types generically, so `tool.mounted` already appears with its payload in
  the run inspector. A dedicated UI affordance (e.g. a badges row on the
  run header) can follow if wanted.
- Mounts still do not survive restart (memory-only registry); that
  boundary stays with TT-1/TT-2 as documented in
  `docs/logs/2026-09-02-tt2-resume-mounts/`.

## Verification

See `verification.md` for commands and outcomes.

## Acceptance

See `acceptance.md` for the audit-trail view a human can check.
