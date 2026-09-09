# UI-AUDIT-LIFECYCLE-HOME: make the species-side lifecycle page read-only inspect

## Scope

`VIVY-STUDIO.md` has ruled (NG-23/NG-28, §258) that Studio is authoritative for evolution
and the species side retains only read-only `inspect`; species-side
Generation/EvalRun/Promotion is the "wrong home", retained frozen in code while product
authority has moved. A 2026-08-31 review found that everyday Vivy's `/lifecycle` still
provided create/reject/startEval/record/promote write forms and buttons, contrary to the
ruling.

- `LifecycleView.tsx` is rewritten as read-only: remove the Generations create form and
  Reject button, the Evals start/record external-evaluation forms, and the Promotions
  promotion form; retain the Species inspect card and the three read-only
  Generations/Evals/Promotions lists (badges/SHA/actor and so on display as before).
- Add the authoritative note row `lifecycle.readonlyNote` (en/zh) below the header and
  change the subtitle wording to read-only.
- Remove the 30 lifecycle keys from en/zh that served only the forms (confirmed to have no
  other references).
- Leave the generations/evals/promotions write actions in `api.ts`/`store.ts` untouched
  (NG-28 "do not expand product semantics", §258 "retain code until the Studio ledger can
  replace them") — this slice narrows the UI surface only and does not remove backend
  capability.

## Explicitly not done

- Did not remove backend RPCs (`generations/*`, `evals/*`, `promotions/promote`) or store
  actions: retaining them frozen is explicit architecture, and deletion is a separate slice
  that must align with the Studio ledger landing.
- Did not change the "Open lifecycle" link in the Settings card (the inspect entry point is
  legitimately retained).
- Did not change the Evolution page or anything on the Studio side.
