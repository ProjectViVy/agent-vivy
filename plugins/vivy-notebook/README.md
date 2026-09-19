# vivy/notebook

The 记事本 (Notebook) sidebar Module for the Vivy web Face.

- **Module:** `vivy/notebook`
- **Port:** `std/ui-extension@v1` (Provider `vivy.notebook.sidebar`)
- **Scope:** generation (UI-only; no backend Port, no Grant)
- **Owns:** the `/notebook` route, its grouped `vivy` sidebar entry
  (order 50), the page chrome, and the `plugin.vivy/notebook.*` copy.

Selecting it in a Recipe is what makes the Notebook block exist:

```yaml
modules: [vivy/notebook]
order:
  std/ui-extension@v1: [vivy/notebook]
ui:
  sdkVersion: 1.0.0
  extensions:
    - {id: vivy.notebook.sidebar, moduleId: vivy/notebook, port: std/ui-extension@v1, entry: ./ui/vivy-notebook/src/index.tsx, export: extension}
```

Verify with `go run ./sdk verify plugins/vivy-notebook`; build the repository dev
projection with `go run ./sdk stage-ui --recipe recipes/default.vivy.yml
--out ui/src/generated`. The Module is a repository source (T1) registered in
`sdk/internal/frontend_v1.go`, so it needs no Recipe `sources:` pin.

The Module reuses the host UI kit through the `@/` alias
(`@/components/ui/*`, `@/lib/demo-api`) and declares only the packages it
imports by name; those pins must stay exact and installed in `ui/node_modules`.
