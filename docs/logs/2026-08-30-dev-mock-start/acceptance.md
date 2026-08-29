# Acceptance

Offline development starts without an API key.

1. No `OPENAI_API_KEY` / `ANTHROPIC_API_KEY`. From the repo root, `just dev` (or `.\dev.ps1`) prints `using config.dev.yaml (runtime.mock=true)` and listens on `:8787` + `:3015`.
2. Open `http://127.0.0.1:3015`. Sending a message gets a deterministic `mock reply to: …` (or HITL scenario when `mock_scenario` is set), not a wizard / missing-key error.
3. If `127.0.0.1:3015` is already in use, `just dev` still refuses until that Vite is stopped — that is the existing split-loop guard, not this bug.
4. Settings cannot activate mock; only `config.Runtime.Mock` / tests can.
