# Verification

- Before the attribute change, `go test ./internal/modules/masks -count=1` reproduced six failures reporting that embedded definition bodies were not normalized.
- In a fresh Windows worktree, `git check-attr text eol -- internal/modules/masks/prompts/*.md` reported `text: set` and `eol: lf`; `git ls-files --eol` reported `i/lf w/lf` for the prompt files.
- `go test ./internal/modules/masks -count=1` passed in that fresh worktree.
- `just ci` passed in the fresh worktree. The run covered UI typecheck, 400 UI tests, production build and i18n checks, `go vet ./...`, `go test -timeout 20m ./...`, headless compile checks, and plugin/face vet and tests. Final `JUST_CI_EXIT_CODE=0` and process exit code `0`.

The full CI run was performed at commit `fc96d80` in a clean checkout so it included the committed attribute without inheriting generated-file state from the implementation worktree.
