# Acceptance

## GitHub Actions

1. Open the pull request's **Checks** view.
2. Confirm `backend ci` and `ui ci` are separate jobs and neither waits for the
   other.
3. Confirm `backend ci` runs the plugin fixture validator, builds `ui/dist`,
   and reaches the Go formatting, vet, test, headless, and plugin-module gates
   without depending on `ui ci`.
4. Confirm `ui ci` runs UI typechecking, unit tests, the production build, and
   cross-face I18N completeness/conformance checks.
5. Confirm the aggregate status is named `just ci` and succeeds only when both
   component jobs succeed.

## Local Windows check

From a checkout with the pinned Node, pnpm, Go, and `just` toolchains:

```text
just backend-ci
just ui-ci
just ci
```

The first command is the independent backend gate. The second is the complete
UI gate. The third remains the complete repository gate.
