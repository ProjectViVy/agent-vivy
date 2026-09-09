# Acceptance — 2026-08-26 tool polish

How a human can tell it worked, from the product view.

## read_file carries line numbers

Ask Vivy to read a text file in a run workspace (or watch any read_file
tool call in the chat tool inspector). The returned content lines now look
like:

```
1	package main
2
3	func main() {
```

Range reads keep true file numbers (reading lines 40–42 shows `40\t…`,
`41\t…`). Patch workflows can cite these numbers directly.

## echo_info no longer on by default

A fresh `config.yaml` copied from `config.example.yaml` boots without
echo_info in the enabled tool list (species/inspect no longer lists it),
while an explicit `- echo_info` entry still resolves — existing test and
e2e configs keep working.

## network_search tells the truth about its providers

The tool description the model sees now names the env vars per provider
and states that without keys it degrades to DuckDuckGo/Wikipedia.
`config.example.yaml` documents the same under `tools:`.

## network_search is configurable in the UI

Settings → Tools → Network Search:

- Availability badges show which providers are live right now and which
  env var is missing (`bing · Not configured (requires BING_SEARCH_API_KEY)`, etc.);
  setting that env var flips the badge on the next start.
- Pick a default provider (Automatic or a configured one), Save Network Search Settings,
  reload the page — the choice persists (settings document on disk) and
  takes effect on the next launch of the engine. An unavailable preferred
  provider silently falls back to automatic selection instead of failing
  searches.
- Saving from the Model tab / welcome wizard / model switcher no longer
  wipes the network_search preference (whole-document replace now always
  carries it).
