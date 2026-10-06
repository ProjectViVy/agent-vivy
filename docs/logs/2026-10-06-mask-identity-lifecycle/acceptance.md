# Human acceptance

Use a masks-selected packed backend and the split Vite application at `http://127.0.0.1:3015`. Create a conversation, then open VIVY → Masks (面具).

1. Before choosing a mask, the header and current-session card display `Just me` / `我就是我` with an identity icon. The same identity appears in the library. No blank editing form appears as the initial details view.
2. Click a Programmer, Researcher or Writer card to preview its identity and read-only instructions. The current session identity stays unchanged. Click Use mask to apply it; the header and current-session card agree immediately after the committed response.
3. Change the mask from the header's quick selector. The management page reflects the same selection, and reloading preserves the backend selection. During an active reply, the hint explains that the current reply keeps its original role while subsequent messages use the new role.
4. Click New. Name, Description and Instructions appear in a separate editor. Save adds the mask without changing the conversation. Save and use both saves it and applies the committed definition to the conversation that initiated the action.
5. Preview a custom mask and click Edit mask. Save changes its definition. Built-in masks offer Duplicate and edit instead of mutation. Cancel or navigate away with a dirty draft: Keep editing retains it; Discard changes closes or continues navigation. Reloading a dirty editor invokes the browser's native leave warning.
6. Edit the same custom mask in two browser contexts. The second stale save reports a revision conflict and preserves its local draft, including further typing. Load latest version requires deliberate discard and reads the backend again. A failed reload preserves the draft and offers retry.
7. Delete requires confirmation. A mask referenced by the current conversation shows why deletion is unavailable; references in other conversations produce the backend's visible refusal. Change all referencing conversations to another identity, then retry deletion.
8. At 1440, 1024, 390 and 320 pixels, the library and toolbar stay inside the viewport. Mobile selection/details/editing use scrollable bottom sheets with reachable actions. In Chinese at 320 pixels, `我就是我` remains fully readable in the toolbar.
9. The small Code/Life (`代码`/`生活`) button sits immediately left of the mask selector, starts at Code, and toggles its display without changing the selected mask or backend runtime mode.

The browser acceptance suite exercises these paths against disposable backend state. Tenant journals are untouched. Visual checks covered desktop library, desktop editor, mobile details/editor and the Chinese 320-pixel editor. Full verification is recorded in verification.md.
