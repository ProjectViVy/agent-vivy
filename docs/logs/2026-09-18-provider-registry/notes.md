# Notes — why the old provider shape existed, and what it actually was

Written 2026-09-18 at program closeout, after the analysis in the iteration
summary turned out to be too generous to the original design. Everything below
is cited against `ff8a47d` (the lane's base). This is discussion, not a contract.

## The correction

The summarizing claim "the fixtures were the runtime truth" is the wrong
framing. The correct one is blunter: **the live provider source was a directory
that the repository itself documents as test data.**

`fixtures/README.md` said so in its first lines:

> Deterministic fixtures for tests and offline development.
> - `provider/` — provider bundle documents used by the real OpenAI-compatible
>   and Anthropic routes.
> ...
> Fixtures must be real recorded data, never mocks standing in for domain
> records in production paths (PRD §6.2).

And the production path read them anyway:

| Site | What it did |
|---|---|
| `internal/app/app.go:255-270` | at startup, `provider.LoadBundle(filepath.Join(cfg.Providers.BundleDir, name+".yaml"))` for `deepseek`, `openai`, `anthropic`, then `provider.NewCatalog(...)`; a missing file aborts composition ("app: load deepseek bundle") |
| `internal/config/config.go:607`, `config.example.yaml:37` | default `bundle_dir: fixtures/provider` — a path relative to the process working directory |
| `internal/rpc/control.go:4447` + `sdk/tui/live/rpc.go:112` | the same three bundles were serialized to both faces as `bundles` |
| `Dockerfile:39` | `COPY fixtures/provider /app/fixtures/provider` — the container had to carry the data beside the binary |
| `internal/studiocore/service.go:83` | the pack path defaulted `Isolation.BundleDir` to `filepath.Join(opt.Worktree, "fixtures", "provider")` — a copy taken from whichever worktree packed it |
| `internal/eval/isolator.go:74-80` | eval children got `bundleDir` plus the same `"fixtures/provider"` fallback |
| `internal/modules/defaults/providers.go:19-38` | a hand-written Go table restating the same three vendors' model ids and secret names for the compiled Profiles |

There was **no `//go:embed` for provider data anywhere** at that commit (the only
embed in `internal/` is `internal/runtime/marketplace_featured.yaml`).

## Why this is worse than "four copies"

Duplication is a maintenance cost. This was an ownership failure, and it shows
up as three concrete defects that duplication alone does not produce:

1. **The binary had no provider knowledge.** A bare `vivy.exe` could not start
   without a sibling `fixtures/provider/` directory, because the default was a
   worktree-relative path and there was no embedded or fallback source. The
   product's model catalog was a *deployment asset*, shipped by hand (Docker
   `COPY`) or copied at pack time — not part of the artifact's identity.
2. **The evidence never covered it.** `fixtures/` sits outside `internal/`, so
   the source digest and the provider-conformance evidence — the artifacts that
   are supposed to say "this is what was verified" — hashed and attested a tree
   that did not include provider data. Two installs packed from two worktrees
   could carry different provider data under the same binary identity. That is
   why `PROV-P1` moving the data into `internal/provider/data/**` mattered more
   than its diff size suggests: it put the data inside the hash.
3. **Everyone was hand-carrying it, and that was the tell.** The iteration logs
   record the workaround as routine: `docs/logs/2026-08-26-list-dir-tool/
   verification.md:41` ("CWD = temp dir (needed `fixtures/provider/*` copied
   alongside)"), `docs/logs/2026-08-26-execute-timeout-ceiling/verification.md:24`
   ("SQLite path and `bundle_dir` pointed into scratch / worktree"),
   `docs/logs/2026-09-03-face-tui-1-f3/verification.md:3` ("read-only copies of
   repository fixtures/provider"). A system whose smoke runs begin by copying
   its own data next to the working directory has already lost track of who owns
   that data.

## The four descriptions, restated by role

Not four attempts at a source of truth — four consumers each keeping a local
note to itself:

| Consumer | Its copy | Owned by |
|---|---|---|
| runtime | `fixtures/provider/{deepseek,openai,anthropic}.yaml` (borrowed from the test directory) | nobody |
| compiler / Assembly | hand-written `defaults.ProviderProfiles()` | nobody |
| operator | `config.providers.<vendor>.{env_key, default_model}` | nobody |
| UI | generated `provider-catalog.ts`, produced by a Python script from a **gitignored** vendored tree (`ui/agent-diva-source/…/providers.yaml`) | nobody |

The UI's 45-vendor list and the runtime's three-vendor catalog were therefore
never the same data set at all. The setting page showed a catalog the runtime
could not construct, while the runtime read a directory the UI never saw.

## Root cause

**The provider definition was never assigned an owner.** With no owner, each
consumer cached what it needed where it was convenient, and the last place the
runtime's copy landed was a directory named `fixtures/`. Nobody renamed it,
hashed it, embedded it, or gated it, because from each local viewpoint it was
someone else's file.

So the failure mode is not "too many copies". It is: **no owner, and the
accidental path wins.** The accidental path was a test fixture that a startup
call site adopted, and it stayed named "fixtures" for the rest of its life.

The owner's diagnosis in review ("DeepSeek is a provider that inherits the
OpenAI and Anthropic interfaces, it is not a provider itself") attacked the same
structure from the other end: once the sealed unit is the *protocol*, the
vendor stops being an identity, and the copies lose their reason to exist.

## Generalizable checks

1. **A directory name is a contract.** Production data under `fixtures/` will be
   read as production data and treated as sample data at the same time. If the
   live path reads it, rename it in the same change or the mislabel becomes
   permanent.
2. **Ownership is "is it in the artifact", not "is it in the repository".**
   A file that ships beside the binary, is copied by the packer, and is absent
   from the source hash has no owner regardless of where it lives in git.
3. **Evidence only covers what it hashes.** Placing data outside the digest
   makes the evidence attest something it never looked at — the same class of
   error as a hand-maintained constant, one level up.
4. **A routine workaround is a design defect with a support cost.** "Copy the
   fixtures next to the cwd first" should have been treated as the bug report it
   was.
5. **Fix it by deleting copies, not by synchronizing them.** The program's
   answer was one embedded data file, a projection for the compiler, a
   projection for the UI, and a deleted `fixtures/` — three of the four copies
   stopped existing rather than being kept in step.

## Before and after, in one page

Shape at the lane's base (`ff8a47d`): five inputs, four selection vocabularies,
no owner.

```text
Agent-Diva registry (ui/agent-diva-source/…, 861 files, GITIGNORED — not in git)
   └─ ui/scripts/gen-provider-catalog.py  (205 lines, run BY HAND, no gate)
        └─ ui/src/components/settings/provider-catalog.ts  (265 lines, COMMITTED)
             → "Source of truth: Agent-Diva provider registry"
             → 45 display rows, all mapped onto bundle ∈ {openai, anthropic, deepseek}
             → selection = (bundle, baseUrl, defaultModel); baseUrl is the escape hatch

fixtures/provider/{deepseek,openai,anthropic}.yaml   (3 files, ~4.4 KB)
   └─ app.go:255-270  LoadBundle(cfg.Providers.BundleDir + "/" + name + ".yaml")
        bundle_dir defaults to "fixtures/provider" — RELATIVE TO THE CWD
        └─ provider.NewCatalog(...) → rpc "bundles" → Web UI + TUI
        └─ also copied by Docker (COPY), by studiocore pack (from the packing
           worktree), and by the eval isolator's fallback
   (fixtures/README.md: "deterministic fixtures for tests and offline
    development"; PRD §6.2 forbids fixtures in production paths)

config.yaml: providers.active + bundle_dir + deepseek/openai/anthropic
             each with {env_key, default_model}
internal/modules/defaults/providers.go: 3 hand-written Profiles restating
             model ids + secret names; AdapterFamily "openai-compatible"
internal/generated/assembly/zz_default.go: ProviderProfiles ["deepseek","openai","anthropic"]
```

Shape now (`c1da466`): one input, one vocabulary, owned by the binary.

```text
internal/provider/data/vendors.yaml (45 vendors / 47 endpoints / 168 models)
                     + provider.schema.json + README.md
   └─ //go:embed → ParseVendors (KnownFields) → validateVendors → LoadEmbedded
        ├─ Catalog: reconcile data against adapterTable, resolve endpoint + credential
        ├─ AdapterProfiles(): per protocol family, model ids and secrets UNIONED from
        │     the data — the compiled Generation is derived, not restated
        └─ rpc providerCatalogResult → settings/providers.catalog
              ├─ Web UI: catalogRows/projectProviderRow — zero provider data
              └─ TUI: iterates view.Catalog, keyed on the endpoint's adapter

config.yaml: providers.active only (one fallback vendor, membership checked at
             startup where the data is loaded)
settings.yaml: {provider: <adapter>, base_url, default_model}; pre-migration
             vendor names are READ-compatible through one alias table

adapterTable = 3 sealed families:
   openai-completions     SUPPORTED
   openai-responses       DEFERRED-INDEFINITE   (visible, not selectable)
   anthropic-messages     SUPPORTED
```

Quantities, same program:

| | before | after |
|---|---|---|
| Truth sites | 4 (bundles, config, Go table, generated TS) + 1 input outside git | 1 (embedded YAML) |
| Hand-synchronized artifacts | generated TS (265 lines) + 3 Profiles by hand + 3 config blocks + 3 fixture files | none (profiles and payload are projections) |
| Sealed protocol vocabulary | one family name, `openai-compatible` (each vendor implemented it privately) | 3 adapters, capability-stated |
| Vendors / endpoints / models | 3 / 3 / a few dozen | 45 / 47 / 168 |
| UI's legal selection values | `'openai' \| 'anthropic' \| 'deepseek'` + a free-text address | `ProviderAdapterId` × the catalog row's declared endpoint |
| Deployment steps to get data | Docker `COPY`, pack-time copy from the worktree, eval fallback, cwd-relative default | none — `//go:embed` |
| Evidence coverage of the data | none (outside `internal/`, outside the digest) | inside it (five `sourceSha256` entries move with it) |
| "Unavailable" expressed as | absent from the bundle set (invisible) | `DEFERRED-INDEFINITE` state (visible, disabled, badged) |

What deliberately did **not** change: the adapter implementations themselves
(`openai.go`, `claude.go` keep their request/response behaviour), the
`providerprofile` Port contract and its SDK evidence anchors, `config.Validate`
staying shape-only, and read-compatibility for documents that stored a legacy
vendor name.

## How it came to be, and how to catch the next one

Nothing here was one bad decision. The shape was assembled by eight
feature-driven deliveries in three weeks, each individually correct:

| Date | Delivery | What it added to the provider picture |
|---|---|---|
| 2026-08-27 | `custom-provider-registry` | custom providers and models beyond the static catalog — provider facts become *writable user state* |
| 2026-08-27 | `genparams-per-provider` | per-model generation parameters — another provider/model-shaped settings block |
| 2026-08-27 | `provider-address-edit` | "still cannot see where to edit the model address" — the address becomes user-editable, i.e. the `base_url` escape hatch |
| 2026-08-27 | `provider-catalog-fold` | ports Agent-Diva's 45-vendor catalog and fold logic into the frontend — **the borrowed domain model, the vendored tree, and the generated TS array all enter here** |
| 2026-08-27 | `provider-edit-polish` | add-vs-edit dialog semantics on top of that catalog |
| 2026-08-28 | `provider-direct-write` | owner: requiring env-var handling "is a problematic product direction" — provider writes and write-time env sync move into config |
| 2026-09-01 | `vc2-claude-backend` | Anthropic wired through `eino-ext/claude` — the second protocol arrives as a *field on the bundle* (`api_type`/`backend`) |
| 2026-09-16 | `deepseek-default-provider` | DeepSeek becomes the default and "single authoritative provider" — the vendor is promoted to a first-class runtime bundle |

Then the two pressures that could not both be satisfied: the display set
(45 vendors) and the capability set (3 constructible bundles). Everything that
looks strange is a patch holding those two apart.

### The six dynamics

1. **Demand came from the "can the user do it" side; the abstraction was never
   asked to pay.** Every delivery was a legitimate product request, and each one
   needed one more place to remember a provider fact. No delivery's acceptance
   criteria ever included "where does provider truth live", so the debt was
   borrowed passively, one feature at a time.
2. **The wrong path was ten times cheaper than the right one.** Displaying 45
   vendors: generate a TS array from the vendored YAML — an afternoon. Serving
   them from the backend: schema + runtime + config + RPC + UI. Reading provider
   data at startup: three lines (`LoadBundle`). Embedding it: config struct,
   Dockerfile, pack path, eval isolator, digest. When the locally wrong move is
   much cheaper, every iteration takes it, and the bill arrives later as a
   count mismatch.
3. **A borrowed domain model silently defined our vocabulary.** The
   `provider-catalog-fold` port carried a desktop client's provider model with
   it: `api_type`, `keywords`, `is_gateway`, `is_local`, `gateway_prefix`,
   `detect_by_key_prefix`, `detect_by_base_model`, `strip_model_prefix`,
   `env_extras`, `model_overrides`. In that model the vendor is the identity, the
   protocol is a field, a gateway is normal, and the client guesses the vendor
   from a pasted key. Vivy is a single-active-provider runtime that never guesses
   and never rewrites a model id — not one of those fields matches a behaviour we
   have, but the vocabulary stayed, and it is why DeepSeek could only be a
   "special first-class bundle" and why two protocols were unrepresentable.
   Porting a catalog ports its abstractions, and abstractions are harder to
   delete than code.
4. **Every feedback loop asked "does the feature work", none asked "is this
   still one system".** Each delivery had a log, acceptance notes and tests. But
   the digest only hashed `internal/` (`fixtures/` was outside it), `just ci` did
   not include e2e, Docker/pack/eval each copied the data with no cross-check,
   and the tests were themselves consumers of `fixtures/provider`
   (`fixturesDir = "../../fixtures/provider"`). **The test suite was an
   accomplice, not a sentinel.** Nothing in the system could turn red on "there
   are now four copies".
5. **Docs and naming legitimized the defect.** `fixtures/README.md` says
   "for tests and offline development" and PRD §6.2 forbids fixtures in
   production paths. The rule was written down, which is exactly why nobody
   looked: the directory was named `fixtures`, so it must be sample data, and a
   rule existed, so someone must be enforcing it. The rules were true and
   toothless at the same time.
6. **The concept was finally recovered by asking a question, not by a gate.**
   The turn came from the owner asking what the provider actually is, then
   stating the single-source requirement, then diagnosing the vendor/protocol
   confusion in one sentence. Debt of this kind does not surface on its own — it
   gets fished out by re-asking a concept. That makes "re-ask the concept" a
   process to schedule, not a stroke of luck to wait for.

### The cheapest tripwire, in hindsight

At `provider-catalog-fold` the same fact already had two different sizes: the UI
showed **45 vendors**, the runtime could construct **3 bundles**. That single
mismatch was the whole defect in visible form, three weeks early, and it cost
nothing to notice — *when two layers report different counts for the same thing,
stop and decide which layer owns it.* The other two cheap signals were "a field
we never read" (the borrowed schema) and "a workaround performed every time"
(copying `fixtures/provider` next to the cwd before each smoke).

### Countermeasures worth keeping

1. **Every fact that enters the run path must answer three questions**: is it in
   the artifact, is it in the evidence/digest, and who changes it. Two out of
   three is a defect, not a follow-up.
2. **Displayed truth must come from capability truth.** Every row a face renders
   must be declared by the backend, or explicitly marked deferred. This is now
   true (catalog + capability state + the canary check); make it a gate rather
   than a property.
3. **Add a tripwire that turns red on a new copy**: a static check that the run
   path never references `fixtures/`, and that a committed generated artifact
   must ship with a comparison check. The repository already has `scripts/check-*`
   precedent; this is cheap.
4. **Audit borrowed schemas field by field**: for each imported field, "do we
   have this behaviour?" — delete what we do not, and record why (the data
   README records the drops; `provenance` records the origin).
5. **Add one question to every delivery's acceptance**: *did this change give any
   fact a second home?* Near-zero cost, and it catches most of this class.
6. **Treat deletion as a deliverable.** Most of `PROV-P1`'s diff is removals
   (`fixtures/`, `bundle_dir` at five sites, the `Bundle` type, the schema, the
   generator, the array). If removing a copy earns no credit, copies only
   accumulate.
7. **Schedule the concept review.** After the third delivery in one domain,
   force the question "how many things does this word mean now?" — the trigger
   this time was a human asking, and that is not a mechanism.