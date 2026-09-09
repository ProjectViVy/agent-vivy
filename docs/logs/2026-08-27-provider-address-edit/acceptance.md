# Acceptance steps (user perspective)

Prerequisite: open `http://127.0.0.1:3015` (not the embedded 8787 page); if it is an old
page, **hard refresh** (Ctrl+Shift+R) first.

1. Go to **Settings → Model**. Click any catalog provider on the left (for example,
   OpenAI / DeepSeek); the right header shows the provider name and **Base URL address text,
   with a pencil icon to the right of the address**.
2. Click the pencil → the 「Add custom provider」 dialog opens, with the provider's display
   name / runtime bundle / address / default model / model list **pre-filled**.
3. Change the address to your own gateway address → **Save** → a new entry appears in the
   「Custom」 area on the left, and the right side automatically shows the new address and
   its model list.
4. Click any model under the new entry → it immediately becomes the current runtime
   configuration (the top bar synchronizes to the new provider and model).
5. Click the right-header pencil again → the dialog title is now 「Edit custom provider」;
   **Alias** and **Address** can be changed, and saving updates them.
6. Reverse check: change the address back to one already present in the catalog and save →
   it is blocked with the literal "That Base URL already exists…" message—this is the existing
   conflict-prevention rule and is expected.

Expected result: any provider (including first use before any custom entry has been added)
has an edit entry **directly visible and clickable** beside the address text on the right,
and the model address can be changed successfully.
