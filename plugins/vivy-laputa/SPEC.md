---
id: laputa
type: module-design
status: draft
title: laputa (`plugins/vivy-laputa`)
parent: architecture
---

## Responsibility

The persona authority. A persona is six Markdown files — `MISSION.MD`,
`IDENTITY.MD`, `RELATIONSHIP.MD`, `REDLINE.MD`, `DREAM.MD`, `DARK.MD` — stored as
real documents in a global home (`<LAPUTA_HOME>/personas/main`, default
`~/.laputa/personas/main`), assembled in fixed order into the text that precedes a
model request.

It owns storage, validation, revision history, publication and rollback,
detection of edits made outside the agent, the first-run onboarding that extracts
the six files from a description, and `/persona show|update|rollback`.

## Boundary

- It owns persona content and its publication, and nothing else: it knows no
  provider, task, transport or tool.
- It uses `rig-memory`'s directory-files abstraction for storage, so the files stay
  ordinary files a human can edit.
- The home is global, not per project or per session: coding and persistent
  species on one machine share one persona.

## Invariants

- The assembly is the six files in fixed order; empty bodies are skipped and
  nothing is trimmed. A file over 32 KiB, or an assembled persona over 64 KiB, is
  rejected.
- Publication is atomic and revisioned: an update names the revision it replaces,
  carries its update id and a digest of its sources, and lands behind a writer
  lock. A revision snapshot is immutable, so rollback is a real revision.
- `DREAM.MD` and `DARK.MD` are optional; the other four are required.
- An edit made to the files outside the agent does not advance the revision. It is
  reported as unpublished rather than silently served as the published persona.
- Internal roles marked `PersonaExcluded` — AutoDream, Recap — do not inherit the
  conversational persona.
- Injection wraps the request's system role when it is empty and precedes it
  otherwise, so a persona never replaces an explicit system prompt.

## Limitations

- A persona is shared across species and workspaces on one machine; there is no
  per-project persona.

*Unconfirmed:* whether the onboarding extraction's fallback (a random name and the
built-in persona when the description is unclear) is the intended long-term
behaviour or a provisional one; the code and the README agree on the behaviour, the
intent is not recorded.
