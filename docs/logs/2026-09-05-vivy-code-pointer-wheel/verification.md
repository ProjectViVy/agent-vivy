# Verification

## Automated

- `go test ./sdk/tui/view ./internal/tui/view` — passed.
- `cd faces/tui; go test ./...` — passed for the packed face and its shared surface/view wrappers.
- `just ci` — passed: formatting, UI typecheck and 201 tests, production build, Go vet/full tests, headless compile, and every plugin/face module.

## Interaction contract

The shared view regression covers hover-before-click, explicit sidebar focus, pointer movement back to chat, overlay isolation, and compact layout. Both first-party terminal faces consume this shared view implementation.
