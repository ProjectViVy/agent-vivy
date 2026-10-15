# First-run persona onboarding

Date: 2026-10-15
Theme: a guided, emotional first-run initialization of the persona authority,
plus the companion's face.

## What changed

### The first-run panel is the Module's, not the shell's

- `sdk/ui/src/module.ts` gains `ShellOnboardingContribution`
  (`slot: "shell.onboarding"`, `render()`) and its shape guard, so a Module can
  declare presentation into a shell-owned slot the same way
  `ChatHeaderContribution` already declares into `chat.header`.
- `ui/src/plugins/presentation-host.tsx` adds `ShellOnboardingSlot`, which
  renders registered contributions inside the existing error boundary.
- `ui/src/routes/_layout.tsx` renders that slot only when a host exists, the
  layout is initialized and the host welcome flow is closed. The shell owns
  *when* the slot exists; the Module owns *whether* anything appears.
- `plugins/vivy-persona` registers a gate into that slot
  (`ui/vivy-persona/src/gate.tsx`): it asks its own authority
  (`diva.cognitive.status`) and shows the panel only for `uninitialized`. No
  session, an unavailable authority, `ready`, or `incomplete` all render
  nothing.

### One guided implementation, two mounts

`ui/vivy-persona/src/onboarding.tsx` walks 认识 → 面容 → 它是谁 → 关系 → 关于你
→ 世界 → 边界 → 完成 with a clickable progress rail. It is mounted twice: as
the first-run gate and as the uninitialized body of `/persona`
(`view.tsx`), which deletes the previous five-textarea form. The five required
documents are written in one atomic `diva.cognitive.persona.initialize` call;
the client never re-implements Laputa's cap, NFC or whitespace rules.

A rejected write keeps every answer, shows the authority's own message, and
returns to the document that message names. A missing answer returns to the
first empty page instead of asking the authority to reject it. Deferring the
gate is a local courtesy flag (`vivy.persona.onboarding.dismissed`); the page
still offers initialization.

### The companion's face

- `ui/src/hooks/use-companion-avatar.ts`: one stored preference
  (`vivy.companion.avatar`) with an emoji preset or a downscaled photo
  (256×256 centre crop, WebP, ≤120 KB data URL, ≤5 MB input), guarded against
  poisoned or unavailable storage.
- `ui/src/components/layout/CompanionAvatar.tsx`: one renderer plus one
  chooser, so the header button and the onboarding face step cannot drift. The
  face is presentation only: it is never sent to the backend, never enters a
  prompt, and never touches the Journal.

### Legible persona write failures

`internal/modules/diva-cognitive/dispatch.go` now classifies
`laputa/persona` errors through `persona.CodeOf` (cap, invalid content,
revision conflict, …) and keeps their message, instead of collapsing them into
`operation internal`. `sdk/ui/src/module.test.ts`, `ui/vitest.config.ts`,
`ui/src/i18n/{zh,en}.ts`, `scripts/i18n-cross-face-contract.json`,
`docs/architecture/VIVY-PORT-CATALOG.md` and the Module's
`i18n/catalog.json` (73 units) carry the contract, the copy and the checks.

`plugins/vivy-persona` is re-pinned to its recomputed source digest
(`d626d1aa…`); `ui/src/generated/assembly.ts` is regenerated from the Recipe.

## Explicitly not done

- No backend persona schema, action, or Port change: initialization is the
  existing `diva.cognitive.persona.initialize` action.
- No repair path for an `incomplete` persona (recorded in `docs/TODO.md`).
- No cross-device avatar sync, no upload, no avatar in any prompt.
- No client-side cap meter or character counter: the authority stays the only
  judge of what fits.
- No separate display name: it would duplicate `IDENTITY.MD`.
- No new animation library and no focus-trap dependency; the panel follows the
  existing `role="dialog"` and `tw-animate-css` idioms.

## Files

New: `plugins/vivy-persona/ui/vivy-persona/src/{onboarding.tsx,gate.tsx,
gate.test.tsx,dismissal.ts,face-state.ts,kinds.ts}`,
`ui/src/hooks/use-companion-avatar{,.test}.tsx`,
`ui/src/components/layout/CompanionAvatar{,.test}.tsx`.

Modified: `plugins/vivy-persona/{module.go,vivy-module.yaml,README.md,
i18n/catalog.json}`,
`plugins/vivy-persona/ui/vivy-persona/src/{index.tsx,view.tsx,view.test.tsx}`,
`sdk/ui/src/{module.ts,module.test.ts}`,
`ui/src/{routes/_layout.tsx,plugins/presentation-host.tsx,
plugins/presentation-host.test.tsx,components/layout/WelcomeWizard.tsx,
i18n/zh.ts,i18n/en.ts,vitest.config.ts,generated/assembly.ts}`,
`internal/modules/diva-cognitive/{dispatch.go,actions_test.go}`,
`scripts/i18n-cross-face-contract.json`,
`docs/architecture/VIVY-PORT-CATALOG.md`.
