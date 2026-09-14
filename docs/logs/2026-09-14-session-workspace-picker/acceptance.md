# Acceptance

A human can tell this worked at `http://127.0.0.1:3015` when:

1. The chat input footer shows **Default workspace** immediately to the right of
   the context-token percentage for a session without an explicit directory.
2. Clicking that entry opens **Choose a workspace**, where only directories are
   listed and the user can move to a parent/root or enter an absolute path.
3. Clicking **Use this folder** closes the dialog and changes the input-bar label
   to the selected directory name; its tooltip contains the canonical full path.
4. The session moves under the matching directory heading in the middle sidebar.
   Sessions with no selection remain under **Default workspace**.
5. Choosing another workspace from a chat with history creates a new selected
   chat rather than re-pointing historical runs at a different filesystem.
6. Creating a chat from a sidebar workspace heading attaches that exact workspace
   to the new chat.
7. Confirming the current workspace leaves the current chat selected, and a
   manually typed path cannot be used until Vivy has successfully browsed it.

The previous arbitrary folder create/rename/pin/remove controls are intentionally
absent: workspace categories now represent actual runtime directories.
