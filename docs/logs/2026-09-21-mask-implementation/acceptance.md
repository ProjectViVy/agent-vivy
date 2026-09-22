# Implementation acceptance

Reviewers can inspect the feature branch and verify that a sealed generation
with the mask capability admits message, run, start event, prompt snapshot,
and optional edit marker through one storage transaction. The run.started
prompt marker and checkpoint envelope carry the snapshot identity; resume paths
load and validate that identity before Eino receives opaque continuation state.

The runtime does not hold the mask Manager. App copies `PromptAssets` once and
passes those values beside the `maskcontract.Resolver`; omitted generations
therefore retain the explicit unmasked path while selected generations fail
closed when required storage or mask assets are unavailable. The first-party
App no longer silently falls back to sequential primary admission in a sealed
composition.

The UI slice is removable: `vivy/masks-ui` owns its route, selector, editor,
action client and localization, while the host owns the typed header slot and
the shell's independent code control. The Module is not yet part of the default
Recipe, so omission and selected Generation artifact claims remain open.

Release acceptance is not yet complete: MASK-4 must supply backend-driven UI,
independent code controls, selected/omitted/backend-only artifact evidence,
split browser smoke, and the final CI/model evidence. The focused permanent
mask-entry rule edit is intentionally untouched pending its explicit approval.
