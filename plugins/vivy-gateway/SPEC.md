---
id: gateway
type: module-design
status: draft
title: gateway (`plugins/vivy-gateway`)
parent: architecture
depends-on: [basics, process]
---

## Responsibility

Where models come from. It turns configuration into registered provider
instances, exposes their model catalog, holds credentials, and edits all of it
from the terminal: add a preset or a custom OpenAI-compatible/Anthropic-compatible
endpoint, set a model's facts (context window, output limit, tools, images,
reasoning levels, prices), choose a credential source, and save.

## Boundary

- It owns configuration and registration, not protocols. Wire formats, dialects
  and option mapping stay in Rig; a preset reuses Rig's built-in metadata instead
  of restating it.
- Builds on `basics` for the shared status surface
  and on `process` for credential filtering.
- It is not a provider catalog for its own sake: only instances the user added
  enter the model catalog.

## Invariants

- **Only the user's settings are read** — never a project's `.rig/settings.toml` —
  so a cloned repository cannot redirect the key.
- Model identifiers are sent to the endpoint unchanged; the module performs no
  rewriting of its own.
- An explicit credential source never falls back to another account. Saved keys
  live under `RIG_HOME/auth/provider-keys/`, outside settings and sessions.
- Secrets are redacted from errors, logs and protocol output.
- Request options that need a dialect are refused without one, because Rig maps
  them per dialect.
- The older `[gateway]` table stays usable as `gateway/<model>` with
  `RIG_SECRET_GATEWAY`; a provider save migrates it into the instance collection,
  and an explicit `providers.instances.gateway` keeps priority.
- An invalid settings file keeps the last valid catalog; an external edit
  conflicts with an open draft rather than being overwritten.

## Limitations

- Subscription login and the Responses API are not included; model discovery is
  manual.
- A preset asserts a known compatibility mapping, not verified compatibility with
  an endpoint nobody tested.
- The legacy `[gateway]` path is a compatibility surface; new configuration is
  expected to use provider instances.
