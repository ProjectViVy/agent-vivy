# Acceptance

1. Start the split development pair (`just run` and `cd ui; pnpm dev`) with no
   provider key, open `http://127.0.0.1:3015`, create a session, and send
   `Hello`. The run ends in a failure card/event reading exactly
   `Unable to connect! Check the provider configuration!`; no `mock reply` text appears.
2. Configure DeepSeek in Settings → Model using the OpenAI-compatible bundle,
   DeepSeek Base URL/model, and an API key (or `OPENAI_API_KEY` plus the
   corresponding `VIVY_API_BASE`/`VIVY_MODEL` environment overrides). Send the
   same message again. The response is the provider's live response; the
   former deterministic `mock reply to: ...` text is impossible because the
   mock provider and fallback have been removed.
3. The model settings provider list contains real OpenAI-compatible/Anthropic
   catalog entries only; there is no selectable Mock provider.
