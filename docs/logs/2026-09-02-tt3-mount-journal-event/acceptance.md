# Acceptance — TT-3 mount journal event

## How a human can tell it worked

1. Run a session where the agent calls `skill_view` on a skill whose
   SKILL.md declares tools (any skill with a `tools:` frontmatter list).
2. Open that run in the Run Inspector (run event feed) and page to the
   point right after the `skill_view` call completes.
3. A new `tool.mounted` event appears between the `skill_view`
   `tool.finished` and the following model turn, with a payload naming the
   mounting tool (`tool_name: "skill_view"`) and the exact tools the skill
   activated (`tools: [...]`) — previously this transition was invisible in
   the journal.
4. Re-viewing the same skill later in the same run emits nothing new (the
   registry dedups), and a failed `skill_view` (unknown skill) emits
   nothing — the event records real mount deltas only.

## Where else it shows up

- Journal replays (recovery, audit exports, `/rpc` run event streams) carry
  the event like any other run event; the schema is versioned under
  `schemas/events/payloads/tool.mounted.json` v1.
