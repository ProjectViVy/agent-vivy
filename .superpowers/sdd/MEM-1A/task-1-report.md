# MEM-1A Task 1 report

## Changed files

- `internal/modules/memory/module.go` — `NewModule()`/`Construct` owner for both
  catalog records, `Open(ctx, cfg)` composition entry over `bml.NewHome` +
  `Warmup`, package-level `active atomic.Pointer[Service]`, `Active()`,
  `Close()`.
- `internal/modules/memory/service.go` — `Service` wrapping `*bml.Home` with
  `List/Get/Search/Add/Update/Remove/Rules/WriteRules/Status`; bml `HomeError`
  codes map to contract outcome reasons (`bml_unavailable`,
  `memory_revision_conflict`, `memory_not_found`, `memory_invalid_request`)
  without leaking record Content into error strings.
- `internal/modules/memory/provider.go` — sync-plane skeleton: `NewProvider()`
  returns one `*Provider` satisfying `contextsource.Provider`,
  `observer.RunProvider`, and `observer.ReceiptRunProvider` under the single ID
  `vivy.memory.bml`. Resolves `Active()` at call time; closed registry yields
  `errUnavailable` + `DeliveryFailed` receipt, live store yields
  `errNotImplemented` (real recall/ingest land in Task 3).
- `internal/modules/memory/actions.go` — `ActionProviders()` returns the closed
  inventory of 9 `vivy.memory.*` control-action providers owned by
  `vivy/memory-bml`. Invoke returns explicit `bml_unavailable` (closed) or
  `unsupported` (live, until Task 2 wires dispatch) outcomes; input/output caps
  match the masks precedent (32 KiB / 256 KiB).
- `internal/modules/memory/module_test.go` — `Open` on `t.TempDir()`,
  Active/Close lifecycle, unavailable-outcome when closed, and
  unavailable-outcome across all 9 action providers without an open service.
- `internal/modules/defaults/catalog.go` — `RunObserverProvider` flag on
  `Binding`; `vivy/memory-bml` record (9 control-action PortRefs,
  `ProviderConstructor=ActionProviders`, `ProviderCollection`) and
  `vivy/memory-bml-sync` record (`context-source` + `observer/run` PortRefs at
  `vivy.memory.bml`, `ProviderConstructor=NewProvider`, `RunObserverProvider`),
  Requires on `core/context-host@v1` + `core/observer-host@v1`.
- `sdk/internal/cmd/generate-default/main.go`,
  `sdk/internal/frontend_v1.go` — propagate the new internal Binding
  `RunObserverProvider` flag into `assemblyv1.GoBinding` at both catalog
  conversion sites. Previously no internal record provided
  `std/observer/run@v1`, so the flag had no internal mapping; the generator
  requires it (runtime_generate.go).
- `recipes/default.vivy.yml` — appended `vivy/memory-bml` +
  `vivy/memory-bml-sync` to `modules:`; no `order:` entry.
- `internal/app/app.go` — `memorymodule.Open(ctx, cfg)` after
  `storagemodule.Open`, gated on `assemblyHasModule(..., "vivy/memory-bml")`,
  with owned-flag error-path close; `memorymodule.Close()` joined into
  `App.Close()` before backend close.
- `go.mod` — `require github.com/ProjectViVy/agent-vivy/bml v0.0.0` +
  `replace => ./bml`.
- `bml/home.go` — added `Home.SearchVisible(ctx, SearchQuery)`: the read-side
  FTS5 recall accessor the plan's `Service.Search` requires (store method was
  private). Forces the machine-home scope; missing store returns empty hits.
- `internal/generated/assembly/zz_default.go` — regenerated via
  `go generate`: `ActionSets` gains the `vivy/memory-bml` ProviderSet,
  `ContextSources`/`RunObservers` gain `memory.NewProvider()`, manifest sealed
  lists updated.
- `internal/app/default_generation_test.go` — ContextSource inventory
  assertion extended for `vivy.memory.bml`; RunObserver inventory assertion
  added.
- `sdk/internal/testdata/default-generation.expected.json` — golden manifest
  updated (modules, actions, contextSources, runObservers).

## Decisions

- Two records, one package, per Global Constraints: `vivy/memory-bml` owns the
  control-action collection; `vivy/memory-bml-sync` owns the single sync
  provider whose `ID()` = `vivy.memory.bml` satisfies both manifest lists.
- Skeleton providers fail honestly: `bml_unavailable`/`DeliveryFailed` when the
  service registry is empty, `unsupported`/`not implemented` when live but not
  yet wired — never fake success.
- `defaults.Binding.RunObserverProvider` added instead of widening the
  conversion sites to derive the flag from descriptors: keeps internal catalog
  bindings explicit and consistent with `MaskFactory`/`ContextSourceProvider`.

## Commands and results

- `go build ./internal/modules/memory` — pass.
- `go build ./internal/app` — pass (with `ui/dist/.keep` placeholder, the
  documented fresh-checkout constraint for `go:embed all:dist`).
- `go test ./internal/modules/memory -count=1` — pass.
- `go vet ./internal/modules/memory` — clean.
- `go test -tags vivy_headless ./internal/app -count=1` — pass (baseline
  inventory + compose tests green against regenerated assembly).
- `go test ./internal/modules/defaults -count=1` — pass.
- `gofmt -l` on all touched trees — clean.
- `go test ./sdk/...` — pack/UI-build cases fail on missing
  `ui/node_modules/vite` (pnpm install not run on this box); pre-existing
  environmental, unrelated to this change.

## Concerns

- None blocking. The two skeleton entry points deliberately return explicit
  unavailable/unsupported outcomes; Tasks 2–3 replace them with real
  action dispatch and recall/ingest.
