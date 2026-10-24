# Verification

All commands run from the worktree
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/search-diagnostics`
(branch `fix/search-diagnostics`, base e858f878), with the pinned toolchain.

## Run

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.4 gofmt -l internal docs` | no output (clean) |
| `GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/ ./internal/tools/` | pass |
| `GOTOOLCHAIN=go1.26.4 go build ./...` | pass |
| `GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/ -run Search` | `ok agent-vivy/internal/runtime 1.074s` |
| `GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/tools/` | `ok agent-vivy/internal/tools 2.801s` |
| `git diff --check` | clean |

The full `./internal/...` suite was deliberately not run in this lane: a
concurrent lane is testing on the same 16 GB machine and full-suite load
causes unrelated timing flakes. The owner runs the full suite afterwards.

## Unrun lanes

- `just wiring-ci`, `just kernel-ci`, `just d02-ci`: **unrun**. They require
  an independent Linux-filesystem WSL/Linux checkout with its own pinned
  Laputa; this macOS host cannot certify them.

## Pending: browser smoke (owner steps)

Status: **pending** — not executed in this lane.

1. Start the split pair: `just dev` (or `just run` plus `cd ui; pnpm dev`),
   open `http://127.0.0.1:3015` (not the embedded UI on `:8787`).
2. Ensure no search key is configured (`BING_SEARCH_API_KEY`,
   `GOOGLE_SEARCH_API_KEY`, `GOOGLE_SEARCH_CX`, `SEARXNG_SEARCH_URL` all
   unset/empty).
3. In chat, ask the model to run a web search with an explicitly named
   keyed provider (e.g. "search the web using bing for ...").
4. In the conversation trace, confirm the tool result is a recoverable
   failure whose text names the provider and the env knob, e.g.
   `network search: provider bing failed: bing search API key is not
   configured: set BING_SEARCH_API_KEY`.
5. Optionally confirm a non-2xx path reads
   `network search: provider <name> failed: search provider <name> returned
   HTTP <code>`.
