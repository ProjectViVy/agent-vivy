# Mask UI restoration

Restore the mask library's card grid, colored role identities, descriptions,
and adjacent detail editor. The original card library landed in `89f301c9`
and its localized presentation in `13f03f49`; default enablement in
`79c0b057` replaced that experience with a form-style session selector.
The restored presentation uses the current removable `vivy/masks-ui` Module
and backend definitions rather than reviving the old static prompt catalog.

The compact mask selector now sits beside the model selector inside the
shell Layout's actual toolbar. PresentationHost supplies its existing live
composition runtime through context; it no longer inserts a selector above
the entire viewport. The library remains a separate Module-owned page.

One extension-owned MaskSession projects the authenticated backend catalog
and selection to both surfaces. It reads catalog pages, ignores previous
session responses, sends the committed revision on writes, recovers after
conflicts, and refreshes on focus/reconnect. Same-session refreshes wait for
pending selection writes. Persisted unavailable selections retain their ID
and warning. The page reducer now owns only editor state; the obsolete form
selector and redundant catalog/selection reducer were removed.

Builtin details are read-only, with duplication into a custom draft.
Custom creation, editing, conflict handling, and in-use delete protection
remain on the existing seven `vivy.masks.*` actions. Session durability,
run-captured mask snapshots, policy, model selection, and independent code
mode keep their backend contracts. No agent loop, prompt assets, or Eino
orchestration changed.

Vite directs Tailwind to the active Assembly's staged Module directory,
including SDK temporary staging. Module TSX regression tests now run in the
normal UI gate. The provider remains `vivy.masks-ui.sidebar` on the existing
`std/ui-extension@v1` consumer seam; no new Grants or backend authority path
were introduced. No deployment or push is part of this delivery.
