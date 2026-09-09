# Acceptance — multi-branch merge into main integration record

## User view (acceptance steps)

1. Refresh `http://127.0.0.1:3015/settings`: the settings page simultaneously shows
   real sections for 「General (including the execution timeout ceiling)」, 「Model
   (provider catalog cards)」, 「Tools」, 「Vivy Features」, 「Language」, 「Channels」,
   and 「Network Tools」; the remaining 「Self-evolution / Sandbox」 sections are migration
   previews.
2. 「Channels」 tab: the complete channel configuration UI (card list, edit form, add
   wizard, tutorial dialog) is present, data is stored in `vivy.ui.channels`, and schema
   validation is active.
3. 「Network Tools」 tab: the real network-search preferred-provider selector and
   availability roster are present (DuckDuckGo/Wikipedia are always configured;
   bing/google/searxng appear according to environment variables); after saving,
   `settings/update` persists the settings and they survive refresh.
4. 「General」 tab: the real execution-timeout-ceiling form (blank = the configured
   default of 30s, 0–600 seconds) takes effect at the next startup after saving; the
   generation parameters (Advanced Features, per-model dropdown) remain.
5. After switching models on the chat page or completing the welcome wizard, the network
   search preference and execution timeout are retained (passed through during full-document
   replacement). After switching languages, the 「Language」 tab label follows the UI language.
6. `git log main --oneline` shows the four merge commits; `just ci` is all green.

## Actual-result criteria

- Any tab appearing twice on the settings page (for example, two network-search UIs or
  two channel UIs) = fail.
- The preferred network-search provider being cleared after switching models = fail.
- `just ci` not being all green = fail.
- Branch remnants (unmerged TODOs or duplicate implementations) still being found on
  main = fail.
