# Acceptance — session list header actions

Start the split pair from the repository root (`just dev`, or `just run` plus
`cd ui; pnpm dev`) and open <http://127.0.0.1:3015>.

1. **Header actions are present.** The `Sessions` row in the left sidebar shows
   three icons at its right: search, view options, and a folder-with-plus.
2. **Folder entry works.** Click the folder-with-plus icon. A `Choose a folder`
   dialog opens, already listing the folders of your home directory with
   drive buttons (`C:\`, `D:\`, `F:\`). Browse into any folder, then press
   `Use this folder`.
   - The dialog closes and the app enters the chat for that folder: the
     workspace chip under the composer shows the folder's name, and the session
     appears under a group named after that folder.
   - If the session you were on was still an empty draft, that same draft adopts
     the folder — no second session appears in the list. If it already had
     content, a new session is created in the chosen folder.
   - If the backend refuses the folder, the dialog stays open with a localized
     error; no backend path text is shown.
3. **Search.** Click the search icon: an input replaces the section header.
   Type part of a session title or a folder name — only matching rows remain.
   A query that matches nothing shows `No matching sessions`. Press `Esc` or the
   cancel button: the query clears and the full list returns.
4. **View options.** Click the view-options icon and choose `Flat list`: the
   folder group rows disappear and all sessions render as one list. Choose
   `Group by folder` to restore the grouped list. Press `F5`: the mode you
   picked is still in effect.
5. **No regressions.** The `New session` button in the sidebar body still
   creates an empty session, each folder row's hover `+` still creates a session
   in that folder, and selecting, renaming and deleting sessions is unchanged.
   Opening the picker from the composer's workspace chip still behaves as
   before (it can also select the default workspace).

The three icons and their tooltips are localized in both catalogs: switch the
language in Settings and confirm `搜索会话 / 视图选项 / 选择文件夹并进入`
versus `Search sessions / View options / Choose a folder and enter`.
