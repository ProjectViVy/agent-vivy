# Acceptance — DeepSeek as the default provider

Product view: how a human can tell this worked without reading the diff.

## 1. First launch shows DeepSeek

```text
just dev          # or: just run  +  cd ui; pnpm dev
```

Open `http://127.0.0.1:3015`.

- A fresh profile (no `data/agent-home/settings.yaml`) opens the Welcome
  wizard with **DeepSeek** selected and model **`deepseek-flash`**.
- Settings → Models shows the DeepSeek bundle, and its base URL is
  `https://api.deepseek.com` — **there is no `/v1` suffix**.

If an older profile is in place, the wizard seeds from the backend's
`config_provider` / `config_model`, which now report `deepseek` /
`deepseek-flash`.

## 2. The outbound request is DeepSeek-shaped

The evidence is the offline assertion, so it can be checked without a key:

```text
go test ./internal/provider/ -run TestDeepSeekThinkingRequest -v
```

It pins the exact request body:

- `model: deepseek-flash`
- `thinking: {"type":"enabled"}` together with `reasoning_effort: high` when
  thinking is on
- `thinking: {"type":"disabled"}` when thinking is off

With a live key you can additionally read the wire truth end to end:

```text
VIVY_REAL_SMOKE=1 go test ./internal/app -run TestRealProviderSmoke -race -count=1
```

The provider is a direct DeepSeek endpoint, not an aggregator, so the model id
is sent raw: `deepseek-flash`, never `deepseek/deepseek-flash`.

## 3. A chat turn completes with a real key

With `DEEPSEEK_API_KEY` exported, `just dev` then a plain chat message:

- the reply streams to completion and the run ends with the `run completed`
  badge;
- Settings → Models lists the DeepSeek bundle and accepts selecting it;
- the status/sidebar provider label reads **DeepSeek**.

Without `DEEPSEEK_API_KEY` the organism still starts; it simply reports no
usable model, which is the documented unconfigured state (it does not fall
back to another provider silently).

## 4. The other two bundles still work

`openai` and `anthropic` remain selectable in Settings → Models, and
`config.example.yaml` still documents all three blocks. Existing
OpenAI-compatible custom gateways (a non-DeepSeek `base_url` with
`bundle: openai`) behave exactly as before.

## 5. The gate is green

```text
just ci
```

Expected: `fmt-check`, `ui-ci` (typecheck + vitest + i18n completeness +
build), `vet`, `test`, `headless-compile`, and `plugin-ci` all pass.

One caveat a reviewer must not miss, with its reason in `verification.md`:
the `test` and `vet` recipes currently skip exactly one package,
`agent-vivy/internal/workflow`, which is another lane's untracked
work-in-progress that does not compile. Everything else runs in full.

## 6. Known limitation to expect

DeepSeek reasoning output is **not** yet visible. Reasoning models return a
`reasoning_content` field that the pinned EinoExt OpenAI adapter parses but
never maps into `schema.Message`, so reasoned text is neither streamed nor
persisted. Outgoing `thinking` / `reasoning_effort` control works and is
tested. Tracked as `DEEPSEEK-REASONING-CONTENT` in `docs/TODO.md` §0.1.
