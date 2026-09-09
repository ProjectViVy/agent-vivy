# 2026-08-25 Sidebar Skill-item “masking” bug fix + Evolution placeholder entry

## Problem

The left sidebar had two navigation items pointing to the same `/skills` route:

- Evolution (Vivy group, `nav.evolution`)
- Skill (Tools Management group, `nav.skill`)

When entering `/skills`, TanStack Router marked both `<a>` elements `active`, so
Evolution and Skill were highlighted simultaneously (`bg-sidebar-accent`). After
the user clicked Evolution (the other item), the Skill item was also covered by
the highlight background—“clicking another item automatically masks the SKILL item.”

Root cause: both `VIVY_ITEMS` and `TOOL_ITEMS` in
`ui/src/components/chat/ConversationSidebar.tsx` registered `to: '/skills'`,
while there is only one `/skills` route (the SkillsView management page), so both
necessarily matched the highlight check (`pathname.startsWith(item.to)`).

## Changes

- `ui/src/components/chat/ConversationSidebar.tsx`:
  - Keep Skill as the only navigable `/skills` entry (Tools Management group).
  - Add Evolution back to the Vivy group as a **placeholder entry**: render it
    as a non-navigation `<button>` (not a `<Link>`) with a Planned `Badge`; on
    click, show a notice following the app’s existing “not implemented” convention
    (disappears automatically after 1.8s), without navigation or highlight
    matching. Add a `pending` flag and a local `showNotice` notice to this
    component (the same pattern as ChatInput’s unimplemented button).
  - Import the `Badge` component; restore the `Dna` icon import.
- `ui/src/i18n/zh.ts` / `ui/src/i18n/en.ts`: restore `nav.evolution`, and add
  `nav.evolutionPending` (Planned) and `nav.evolutionUnavailable` (Evolution is
  not implemented yet).

## Design rationale

The product-side Evolution/AutoDream capability remains DEFERRED in
`docs/TODO.md` §0.1 and has no dedicated route. Therefore Evolution is not made
into a navigation link (linking to `/skills` would reintroduce the double
highlight, while linking to a nonexistent route would 404). It remains in the
navigation with a Planned badge, and clicking it gives the same notice as the
  unimplemented `ChatInput` button.

## Explicitly not done

- On mobile, clicking the navigation item for the current route does not close
  the drawer (`pathname` does not change, so `_layout.tsx`’s
  `useEffect([pathname])` does not fire). This independent edge case is outside
  this report and was not changed.
- No `/evolution` route or placeholder page was created—Evolution is not
  implemented; add it when the capability is implemented.
