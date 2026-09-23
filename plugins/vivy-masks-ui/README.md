# vivy/masks-ui

This removable UI Module owns the mask catalog/editor page and session-bound
selector. It invokes the authenticated `module.action.invoke` transport with
backend owner `vivy/masks` and the seven `vivy.masks.*` actions.

The UI keeps the active session and selection behind an epoch/CAS reducer. A
late response from a previous session is ignored, selection writes carry the
server revision, and a custom editor draft remains intact across a revision
conflict reread.

The Module source is intentionally independent from the internal `vivy/masks`
backend source tree. A Recipe must select this UI provider separately and the
Source Catalog must register `plugins/vivy-masks-ui`; the current compiler's
production `vivy/masks` selection gate remains unchanged until its dedicated
release task is authorized and completed.
