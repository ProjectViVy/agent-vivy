# Acceptance guide — 2026-08-27 provider-catalog folding port

Confirm that the feature works from the user's perspective (development environment:
`just dev`, open `http://127.0.0.1:3015`):

1. **See the diva provider catalog**: go to 「Settings → Model」. The left side of the
   "Vivy model configuration" card shows providers (OpenRouter, Anthropic, OpenAI,
   DeepSeek, Zhipu AI, DashScope…27 common providers), with a 「Search providers」 input
   at the top; the provider row matching the current runtime configuration has a 「Current」
   badge.
2. **Infrequent providers folded by default**: the bottom of the list has a dashed 「More
   providers」 row (with count 20 and an arrow). CherryIN, 302.AI, PPIO, Together AI, Yi,
   and others—20 providers in total—are hidden by default; click the row to expand/collapse.
3. **Folding yields to search**: type "yi" in the search box → CherryIN and Yi (01.AI) are
   displayed flat and the folded row disappears; clear the search to restore the common +
   folded structure.
4. **Selecting a provider automatically expands folding** (if the current provider is in the
   folded group, the list expands after refresh to keep it visible).
5. **Selection maps, save uses real settings**: click DeepSeek → its model list appears on
   the right, and the form below becomes Provider=openai, Base URL=https://api.deepseek.com/v1,
   default model=recommended value; clicking a model changes only the default model; click
   「Save real settings」 and the top-bar switcher shows "DeepSeek | model name".
6. **Top-bar switcher follows**: click the top-bar model button; the dropdown lists the
   current provider's models (the current item under 「Current configuration」, the rest under
   「XX available models」), and selection saves immediately.
7. **Custom combinations are unaffected**: manually change Provider/Base URL to a value
   outside the catalog (such as a private gateway); the right side shows a "not in catalog"
   notice and still saves the entered values.

As before, keys are managed only by the runtime environment; the page has no key input.
