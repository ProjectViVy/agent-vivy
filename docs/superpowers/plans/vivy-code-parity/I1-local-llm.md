# I1 — `coding-local-llm` compatibility module

**Goal:** public module providing local-model provider-profiles + lifecycle control actions + status source. Zero kernel changes.
**Epic:** I. **Requirements:** RQ-LLM.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §8. **Baseline:** `f34f3ce`. Use the `vivy-plugin` skill; `plugins/lsp` is the structural template (descriptor + toolworld provider + proc grant).

## Scope

**Files:** `plugins/coding-local-llm/` — `vivy-module.yaml`, `module.go` (Descriptor: `std/provider-profile@v1` ×4 + `std/control-action@v1` + `std/status-source@v1`), `service.go` (probe/lifecycle), `ui/` optional small status card (`std/ui-extension@v1` optional — decide in-story; omit if it stretches scope).

**Ports:** `std/provider-profile@v1`: `local/llamacpp` (http://127.0.0.1:8080/v1), `local/ollama` (http://127.0.0.1:11434/v1), `local/lmstudio` (1234/v1), `local/vllm` (8000/v1) — all `openai-compatible` endpoint class, `apiKeyEnv` unused (dummy ok). `std/control-action@v1`: `local_llm.status`, `local_llm.discover`, `local_llm.start`, `local_llm.stop`, `local_llm.models`, `local_llm.pull` (ollama only). `std/status-source@v1`: per-server health for sidebar. Grants: `GrantProcSpawn`, `GrantNetworkLocal` (or whatever network grant class exists — check catalog).

## Tasks

- [ ] `local_llm.discover`: probe the four ports concurrently (≤1s timeouts), hit ollama `/api/tags`, llama-server `/v1/models`; return `{server, reachable, models[]}`.
- [ ] `local_llm.start {server, model?}`: spawn `ollama serve`/`llama-server --jinja -m <gguf>` via Env.Spawn with tracked PID handle; refuse if already running; `stop` symmetric. Process supervision stays in-module (pi's router model is equivalent).
- [ ] `local_llm.models`: list GGUF under a configured models dir + ollama `/api/tags`.
- [ ] `local_llm.pull {model}`: POST ollama `/api/pull`, stream progress into result (bounded); llama.cpp manual-download note in result.
- [ ] Status source emits reachable/unreachable + loaded model count; sidebar card optional.
- [ ] Conformance: descriptor digest pinned; actions fail closed when server binary missing (`server_not_installed`, not panic).
- [ ] Tests with stub servers (httptest on the four endpoints) + spawn-path fake; `go test ./plugins/coding-local-llm`; pack via `vivy-sdk` on a test recipe; `just ci`.
- [ ] Commit `feat(plugins): local-llm compatibility module`.

## Boundary

No new wire protocol (OpenAI-compatible only — profiles ride existing adapter). llama.cpp router management is best-effort: spawn/stop/status; GGUF download UX deferred (pi's `/llama` download flow is out of v1 scope; document it).

## Acceptance

With `ollama serve` running, `/model` lists its tags via discover; `local_llm.start` brings a llama-server up and the profile routes a real chat through it.
