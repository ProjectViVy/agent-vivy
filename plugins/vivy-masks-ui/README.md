# vivy/masks-ui

This removable UI Module owns the mask card library, detail editor, and compact
session selector next to the model selector in the shell's chat toolbar. It
invokes the authenticated `module.action.invoke` transport with backend owner
`vivy/masks` and the seven `vivy.masks.*` actions.

One extension-owned `MaskSession` projects the backend catalog and session
selection to both surfaces. It ignores late responses from previous sessions,
carries the server revision on writes, and refreshes after conflicts, window
focus, and reconnect. Selection errors stay visible for retry. Editor drafts
remain page-local. Builtin definitions are read-only; duplication starts a new
custom draft without changing the builtin.

The Module source is intentionally independent from the internal `vivy/masks`
backend source tree. A Recipe must select this UI provider separately and the
Source Catalog must register `plugins/vivy-masks-ui`; the current compiler's
production `vivy/masks` selection gate remains unchanged until its dedicated
release task is authorized and completed.
