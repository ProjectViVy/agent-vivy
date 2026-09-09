# Acceptance (product / user perspective)

## Entry point

Open the app → left navigation 「Settings」 → the tab list shows 「Channels」 (after
「Language」, without a 「Preview」 badge; the remaining Network / Compaction /
Self-evolution / Sandbox sections still carry the 「Preview」 marker).

## Verifiable behavior

1. **Empty-state guidance**: on first entry (or after clearing `vivy.ui.channels`), an
   empty-state card 「No channel configurations yet」 and an 「Add channel」 button appear.
2. **Add channel (wizard)**: click 「Add channel」 → select a platform (Telegram / Discord /
   Feishu / DingTalk / Email / QQ / Neuro-Link card grid with brand icons and descriptions)
   → credentials step (quick-guide panel + required-field validation: 「Next」 disabled
   when required fields are missing; secret fields have a visibility toggle; advanced
   settings collapse) → completion page → the new channel appears in the card grid
   (enabled by default).
3. **Card-view operations**: cards contain the platform icon, status badge (Active /
   Needs configuration), enabled state, and missing-field summary; they can be enabled /
   disabled directly, edited through the wizard, and deleted (removed after confirm).
4. **List view**: switch to 「List view」 → left channel list (enabled/ready state), right
   details: status card (readiness / missing fields), enabled switch, and inline edit form
   (basic + advanced + secret visibility); after a change, the 「Save configuration」 button
   becomes active, and saving returns to the baseline.
5. **Persistence**: saved channels remain after page refresh (`vivy.ui.channels`
   localStorage); secret values do not appear in logs or control-plane responses.
6. **Retired channels**: slack / whatsapp and others do not display or allow editing even
   when present in configuration data.
7. **Language**: after switching the UI language (Settings → Language), the channel-section
   copy follows it (zh / en).

## Criteria

Acceptance passes when all items 1–7 are operable and the copy and interaction align with
the Agent-Diva channel page.
