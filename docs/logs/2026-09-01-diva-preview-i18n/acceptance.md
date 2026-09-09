# Acceptance

Manual verification (without reading code):

1. Run `cd ui; pnpm dev` (with the backend running via `just run`) and open
   `http://127.0.0.1:3015`.
2. Every tab in the top row of the Settings page uses localized copy: General /
   Models / Tools / Vivy features / Language / Channels / Network tools / Sandbox /
   Self-evolution (the last one has a "Preview" badge).
3. On the Self-evolution tab, the description, run frequencies (daily/weekly/
   manual), five action rows (identity document / relationship document /
   commitments record / operating guidelines / deprecation proposal), and the
   confirmation-strategy toggle toast all follow the language switch, with no raw
   i18n keys or mixed untranslated text.
4. In the Agent-Diva preview section of the General tab (chat display, cache and
   runtime status, About Vivy, and the "Compaction configuration has graduated"
   migration note), the
   entire section is English after switching to English, with no mixed Chinese.
5. The execution-timeout card, application-information card, tool-configuration
   card, lifecycle card, and Run Inspector card on the General tab all switch
   their titles, descriptions, buttons, and error messages with the language.
6. In the Chinese interface, the execution-timeout card copy is character-for-
   character identical to before the change (no Chinese regression).

Equivalent automated evidence: `language-setting.spec.ts` (switching and
persistence) plus Playwright's real assertions against the Settings-page
rendered text (see verification.md).
