# Toolkit UI redesign

Iteration: 2026-10-17  
Lane: `feat/toolkit-ui-redesign` (worktree `../agent-vivy-toolkit-ui`, based on
`origin/main` `5715950d`)  
Module: `plugins/vivy-toolkits-ui` (UI Module `vivy/toolkits-ui`)  
Scope: UI only — no backend, RPC, Port, Recipe, or Generation change.

## What was wrong

The Toolkit page had grown into one 490-line component that mixed four
different jobs in a single column: browsing the library, installing from a
source, staging a lifecycle change, and inspecting components. Consequences a
user could see:

- the update/rebind source fields were the same state as the install form, so
  one intent could overwrite the other's input.
- every request set one global `working` flag: a stray refresh blanked the
  whole page and locked unrelated controls.
- first load and refresh were indistinguishable, so the page flickered between
  "loading" and content and could show two `role="status"` regions at once.
- each install could only be reached through that shared form state, so the
  page could not say which Toolkit a staged change belonged to.
- the page rendered its own `<h1>Toolkit</h1>` and subtitle inside
  `toolkit-page`, duplicating the host route header that is already built from
  this Module's `titleKey`/`subtitleKey` (`index.tsx`).
- every label went through a local `tx(key, chinese)` helper whose fallback was
  a hard-coded Chinese string, so states, mutation verbs, MCP runtime states and
  change verbs surfaced in Chinese for `en` users whenever the host had no value
  for the key.

## What changed

The page is now orchestration plus five focused surfaces, all sharing the same
data flow (`client.list` → `client.inspect` → `client.prepare` → publish):

- `ToolkitLibrary.tsx` — catalog epoch/configuration revision, one refresh
  control, one install entry point, the installed rows, first-load skeleton and
  empty state.
- `ToolkitInstallDialog.tsx` — a modal that owns install intent only: source
  kind, location, git ref, preview (name, version, origin, release digest,
  component counts), and its own busy/error state.
- `ToolkitDetails.tsx` — mounted per `install_id`, so identity, lifecycle
  actions, update/rebind source fields and rollback always describe the
  selected Toolkit; the update fields are prefilled from that installation's
  own source.
- `ToolkitCandidate.tsx` — the staged change: target, digest, epoch, operation
  id, per-component `added/changed/removed` before→after, fresh-consent marks,
  blockers and the mutation lock.
- `ToolkitInventory.tsx`, `ToolkitExecutableRow.tsx`, `ToolkitContentDialog.tsx`
  — component inventory with a summary strip, per-declaration host environment
  bindings and executable consent, and a read-only content dialog.
- `toolkit-view.ts` — pure label/derivation helpers (state, operation, change,
  mutation, source kind, MCP runtime, digests, candidate target) so rendering
  and copy stay testable without React.

Behaviour fixes that the redesign made explicit:

1. **First load vs refresh.** First load renders a skeleton with no
   `role="status"`; a refresh keeps the current list, marks the refresh control
   `aria-busy`, and never adds a second status region.
2. **Per-action busy, serialized mutations.** Each mutation spins only its own
   control, and reads, selection and refresh stay available while it runs. All
   mutations still serialize, because every one of them consumes the shared
   catalog identity (epoch + configuration revision) — see the defect below.
3. **Separated intent.** The install dialog and the per-installation update
   fields are independent state, so an install cannot silently retarget a
   staged update and vice versa.
4. **Candidate lifecycle.** A prepared change stays visible until its commit is
   confirmed, the library is refreshed and the installation re-inspected;
   publish and cancel are the only exits.
5. **Copy is cataloged.** Every user-visible string is a literal
   `t('plugin.vivy/toolkits-ui.*')` key; the catalog grew from 112 to 142 units.
   `close`, `mcpHealthUnavailable` and `sourceTitle` are gone because no call
   site uses them.

## Defect found and fixed during browser verification

The first end-to-end run failed at the MCP consent step with the page's own
"not yet confirmed" error. Root cause: with per-action busy, the bindings save
no longer disabled *other* mutations, so the consent request started while the
configuration-revision refresh was still in flight and sent the stale catalog
identity — the backend answered `toolkit: revision_conflict`, which the page
faithfully reported as an unconfirmed consent.

Fix: `mutationsLocked = candidate !== null || pending !== null`
(`ToolkitPage.tsx`) — mutations serialize on the shared catalog identity while
reads, selection and refresh stay free. `ToolkitPage.test.tsx` now pins it with
a test that holds a binding write open and asserts the second mutation cannot
start.

## Test-harness change

`ui/e2e/toolkits.spec.ts` and `ui/e2e/toolkits-accessibility.spec.ts` seed
`vivy.persona.onboarding.dismissed` next to the existing welcome seed: on a
fresh profile the unrelated Persona first-run gate opens a modal over the
Toolkit page and intercepts pointer events (pre-existing flake, unrelated to
this Module). The two consent steps additionally wait for the page's
"Authorization recorded" notice before asserting the badge, so a rejected
mutation fails with its own reason instead of a stale-badge message.

## Explicitly not done

- No Go, RPC, action, Port, Recipe, or Generation change.
- No new dependency, route, or navigation item; the Module still declares one
  `std/ui-extension@v1` sidebar provider.
- No mobile Sheet or nested modal: below `xl` the page is a single column and
  the content dialog is the only overlay.
- No second source of truth for component data: everything renders the
  authoritative inspection projection.
- `ui/src/generated/assembly.ts` (tracked) and the Module `source.sha256` were
  regenerated; `ui/src/generated/ui/**` stays generated and gitignored.
