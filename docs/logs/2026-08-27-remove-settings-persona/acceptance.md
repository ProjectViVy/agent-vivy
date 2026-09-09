# acceptance.md — Remove the 「Persona」 panel from Settings

## Acceptance criteria (user perspective)

1. Open 「Settings」 (sidebar → Settings, or `http://127.0.0.1:3015/settings`).
2. The tab list should be, in order: General / Model / Tools / Vivy Features / Channels
   (Preview) … Sandbox (Preview)—「Persona」 no longer appears.
3. No panel remains for editing 「assistant name / system prompt」 and 「Save persona demo」;
   the `?tab=persona` deep link shows no Persona content.
4. The independent sidebar 「Persona」 page (seven persona Markdown documents) still opens
   and can be edited normally.
5. Model / Tools / Vivy Features and all preview sections are unaffected.

## Relation to previous behavior

- Previously, 「Settings → Persona」 was a local demo panel (`vivy.demo.persona` localStorage),
  separate from the sidebar 「Persona」 page (persona documents); this change removes only
  the Settings panel and retains the sidebar Persona page.
