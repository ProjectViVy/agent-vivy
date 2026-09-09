# Acceptance steps (user perspective)

Prerequisite: `http://127.0.0.1:3015` + hard refresh (Ctrl+Shift+R).

1. In **Settings → Model**, click any catalog provider on the left → in the right header,
   the provider name + address are two plain-text rows; the upper-right sequence is the
   runtime-bundle badge, pencil (Edit), Refresh (Sync from official catalog), and Add (+),
   with the three icon buttons **on one row, the same size, and aligned**.
2. Click the pencil → the dialog title is **「Manage provider」** (not 「Add custom provider」),
   and explains "it becomes a custom provider after saving; the original catalog entry
   remains unchanged".
3. Change the address to your gateway address → Save → a new entry appears in the 「Custom」
   area on the left, while the right shows the new address and its model list; click any model
   there to select it.
4. Click the pencil on the right side of a custom-entry row on the left → the dialog title is
   「Edit custom provider」—the two entry semantics are strictly distinct.
5. At the very bottom of the left side, 「＋ Add custom provider」 → the title remains 「Add
   custom provider」 (empty form), clearly distinct from the Edit/Manage entries.

Expected result: edit-type entries no longer contain the word "Add"; the header button group
is visually uniform.
