# Block 2 acceptance — what a human should see

Start the split pair and open `http://127.0.0.1:3015`:

```
VIVY_USER_HOME=/tmp/vivy-ui-overhaul just run     # terminal 1
cd ui && pnpm dev                                 # terminal 2
```

These checks are about behaviour that moved, not about a colour change; the
visual system is Block 1 and the surfaces are Blocks 3–6.

1. **Session list.** Open the app at 1280px. The sidebar lists sessions under
   their workspace. Hovering a row reveals a pencil and a trash button in the
   row's right edge; the row never jumps, and a long title stays on one line and
   is readable in full from its tooltip. Clicking the row selects the session.
   Clicking the pencil turns the same row into an input; Enter saves, Escape
   cancels.
2. **View menu.** Click the sliders icon in the "Sessions" header. A menu opens
   next to the button, the current layout carries a dot, arrow keys move
   between "Group by folder" and "Flat list", Escape and an outside click close
   it. Choosing "Flat list" removes the workspace rows and the choice survives a
   reload.
3. **History picker.** Insert a reference from the composer's history button.
   The session list inside the picker uses the same row: title on the first
   line, workspace or timestamp on the second, and the selected session is
   highlighted. There is no second row implementation with different padding.
4. **No pseudo-controls.** The chat header no longer shows a Code/Life toggle
   that only changed its own label; the scope comes from the real code-mode
   configuration. Nothing opens a session drawer: the sidebar is the only
   session surface. `sessionDrawerOpen` no longer exists in the store or in the
   module contract.
5. **Dialogs at 390px.** Open "Choose a folder and enter" on a phone width. The
   dialog fits the viewport, its body scrolls, and the confirm button stays
   reachable; nothing is clipped by a fixed height.
6. **Nothing unused.** `ui/src/components/ui` contains only primitives with a
   real importer; `pnpm build` succeeds and the bundle no longer carries
   `recharts`, `cmdk`, `vaul`, `sonner`, `react-day-picker` or the removed Radix
   packages.
