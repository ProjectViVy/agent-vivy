# R1 acceptance

1. Open the split UI on an installed generation with the Notebook module and select an empty section (for example Daily). The entry list shows the empty-section message and the detail shows **Settings** and **Generate**; there is no editor and no second report panel.
2. Open Settings, save a disabled schedule, then Generate. The run reaches a terminal state and the view selects the entry named by the generation output, including when that entry lives in another section. The provenance strip names the series/window and does not claim a model run when the runtime used fallback.
3. Edit and save the generated entry, add a comment, then regenerate from the selected-entry header. Existing revisions/comment behavior is unchanged.
4. With an unsaved draft in a different entry, a completion in the background must not replace the draft or selection.
5. In a generation without the reports module the empty detail shows no report controls, and the generic Notebook flow still works.
