# Restore model resolver and SQLite organism lease (2026-08-30)

## Changes

The mainline `app.go` was wired to "settings.yaml / frozen ENV → construct ChatModel
per request" and calls `TakeOrganismLease` when opening the SQLite Journal, but the
corresponding implementation was not merged into mainline, causing `go build ./...`
to fail:

- `undefined: ModelResolver` / `newModelResolver`
- `undefined: provider.NewResolvingChatModel`
- `backend.TakeOrganismLease undefined`

This iteration restores only the parked implementations required for compilation; it
does not merge the Studio plugin tree or skills UI from the WIP commit, nor the config
refactor that removes `runtime.mock`.

### Core

- `internal/app/model.go`: `ModelResolver`. Frozen-ENV sessions take priority;
  otherwise read the user's `settings.yaml` (including the registry `ActiveKey`).
- `internal/provider/resolving.go`: `NewResolvingChatModel` constructs the underlying
  model from `LiveSpec` for each Generate/Stream call.
- `Ref.Model` now accepts `ModelSpec{ID, APIKey, BaseURL}`; the openai Ref no longer
  reads the key with `os.Getenv`. Mock leaves the product Catalog and remains only as a
  test helper.
- SQLite `TakeOrganismLease`: exclusive `vivy/organism` lease plus heartbeat;
  `Close` releases it. Test `Open` does not take the lease, so tests do not contend for
  the same file.

## Explicitly not done

- Do not merge the entire `25b2eb6` WIP (console / skills UI /
  dsh-plugin-subscriptions).
- Do not delete `config.Runtime.Mock` (the product path no longer uses the Catalog mock,
  but the config field remains).
- Do not change the UI or the Studio shell.

## Changed files

- `internal/app/model.go`, `internal/app/model_test.go`
- `internal/provider/{resolving.go,ref.go,openai.go,mockref.go,catalog.go,doc.go}` and corresponding tests
- `internal/runtime/modeladapter.go` (comment)
- `internal/storage/sqlite/sqlite.go`
- `docs/TODO.md` (UI-MODEL-KEY-SCOPE closed)
