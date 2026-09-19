# vivy/evolution

The 进化 (Evolution) sidebar Module for the Vivy web Face.

- **Module:** `vivy/evolution`
- **Port:** `std/ui-extension@v1` (Provider `vivy.evolution.sidebar`)
- **Scope:** generation (UI-only; no backend Port, no Grant)
- **Owns:** the `/evolution` route, its grouped `vivy` sidebar entry
  (order 30), the page chrome, and the `plugin.vivy/evolution.*` copy.

Selecting it in a Recipe is what makes the Evolution block exist:

```yaml
modules: [vivy/evolution]
order:
  std/ui-extension@v1: [vivy/evolution]
ui:
  sdkVersion: 1.0.0
  extensions:
    - {id: vivy.evolution.sidebar, moduleId: vivy/evolution, port: std/ui-extension@v1, entry: ./ui/vivy-evolution/src/index.tsx, export: extension}
```

Verify with `go run ./sdk verify plugins/vivy-evolution`; build the repository dev
projection with `go run ./sdk stage-ui --recipe recipes/default.vivy.yml
--out ui/src/generated`. The Module is a repository source (T1) registered in
`sdk/internal/frontend_v1.go`, so it needs no Recipe `sources:` pin.

The Module reuses the host UI kit through the `@/` alias
(`@/components/ui/*`, `@/lib/demo-api`) and declares only the packages it
imports by name; those pins must stay exact and installed in `ui/node_modules`.
