# Evolution page

Date: 2026-08-25
Status: complete

## What changed

The sidebar “Evolution” entry changed from a pending placeholder (clicking it
showed a “not yet connected” notice and there was no route) to an accessible
`/evolution` page. The page follows the governance structure of agent-diva’s
`EvolutionView.vue` and Vivy’s existing demo-page pattern (`demo-api.ts` +
`vivy.demo.*` localStorage), sharing storage with the Skills page (a single
source of truth).

Three Tabs, each using a master-detail two-column layout (single column + Back on mobile):

- **Skill**: lists only `evolution_managed` skills. Details include the
  authoritative Markdown (view/edit/save with CAS base-hash conflict rejection),
  enable/disable, hard delete (AlertDialog confirmation, only when
  `can_hard_delete`), a history snapshot list, and full previews by version.
- **Pending Review**: a request list (status badges: Pending Review / Accepted /
  Rejected / Stale) plus details (full proposal, base hash, Evidence/Attestation
  panel). “Accept” compares base_hash with the current authoritative head:
  mismatch marks the request stale and rejects it; a match writes the proposal
  to the authoritative head, preserves the history snapshot, and sets
  `evolution_managed`. If the skill does not exist, accepting creates a home
  skill. “Reject” has a confirmation dialog and is available only for pending
  requests. Includes a “New Request” form (slug/title/reason/declaration/
  proposal Markdown, with required-field validation on the frontend).
- **AutoDream**: a run list (status badges) plus details (trigger/mode/stage,
  start/end times, attempt count, failure reason, input summary, progress-event
  timeline, and a proposal link that returns to the Pending Review tab with the
  proposal selected).

Supporting changes:

- `ui/src/routes/_layout.evolution.tsx`: new route (Persona-style title header + DemoBanner).
- `ConversationSidebar.tsx`: removed the pending branch, notice mechanism, and
  dead `Badge` code; removed the `nav.evolutionPending` /
  `nav.evolutionUnavailable` entries as well (zh/en kept in sync).
- `types.ts`: added AutoDream domain types (run/event/input summary/orchestration
  stage, structurally matching the agent-diva wire types).
- `demo-api.ts`: added `vivy.demo.skill-docs` (authoritative document + history)
  and `vivy.demo.autodream` storage; changed `getSkillDocument` from reading a
  module constant to reading local storage (governance writes now appear in the
  document view); added accept/reject/updateSkillDocument/setSkillEnabled/
  deleteSkill/history read/write functions; seed data contains 2
  evolution-managed home skills, 3 mixed-state requests, and 3 AutoDream runs.
- **Fixed a seed-sharing bug**: `readSkillStore`/`readSkillRequests`/
  `readSkillDocStore`/`readAutoDreamStore` previously returned the module-level
  MOCK array itself when seeding, so in-place governance operations polluted the
  module seed (visible across tests or after reseeding when the cache was
  cleared). They now all write through `seedStore` and return copies.
- `SkillsView.tsx`: localized request-status badges through `skills.status.*`
  instead of the raw enum (they are also visible in the Skills page after demo
  request seeding).
- i18n: added the top-level `evolution` domain, `skills.status`, and
  `demo.evolution` demo copy, synchronized between zh/en.
- New test `demo-api.evolution.test.ts`: seeding, accept writing head + history,
  stale detection, duplicate-decision rejection, CAS conflicts, and the hard-delete
  boundary (built-ins cannot be deleted).

## Explicitly not done

- The kernel’s real Evolution/AutoDream capabilities (MEM-1 remains DEFERRED;
  this page does not connect to `api.ts` RPCs or register new RPC_METHODS).
- The search box from the agent-diva page (no identification value at demo-data
  scale), AutoDream real-time output polling (demo data is terminal records, so
  polling would be a fake operation), or the CodeMirror editor (a Textarea is used).
- The `diva.evolution.*` Settings-page preview copy (it belongs to the Agent-Diva
  migration preview section) was left unchanged.

## Scope

- `ui/src/components/evolution/EvolutionView.tsx` (new)
- `ui/src/hooks/useEvolution.ts` (new)
- `ui/src/lib/demo-api.ts`, `ui/src/lib/types.ts`
- `ui/src/routes/_layout.evolution.tsx` (new), `ui/src/routeTree.gen.ts` (generated)
- `ui/src/components/chat/ConversationSidebar.tsx`, `ui/src/components/skills/SkillsView.tsx`
- `ui/src/i18n/zh.ts`, `ui/src/i18n/en.ts`
- `ui/src/lib/demo-api.evolution.test.ts` (new)
- Reference (unchanged): `C:\Users\Administrator\Desktop\morediva\agent-diva\agent-diva-gui\src\components\EvolutionView.vue`
