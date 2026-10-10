# Acceptance

From a fresh checkout, `just setup` installs the locked Laputa source without manual sibling edits. `just dev -NoBrowser` starts the backend and Vite UI from a cold checkout on Windows. Backend, UI, and selected full-UI browser CI lanes must all pass on the repair commit.

`just test` must execute the full ordinary suite and every memory-loop test, with the App memory loop bound to an inspected DIVA artifact rather than a default Assembly lacking recall. The conformance reproduction gate must confirm the refreshed source identity and all Provider/Host checks. A skipped or absent memory-loop pass is not acceptance.
