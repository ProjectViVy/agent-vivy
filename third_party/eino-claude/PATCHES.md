# Local upstream patch

This package contains the production Go sources from
`github.com/cloudwego/eino-ext/components/model/claude` v0.1.25 and its Apache
2.0 license. The root module replaces only that Claude module with this copy.

The local change gives each streaming worker its own `ConcatMessages` error
variable. Upstream assigned to `Stream`'s named return `err` from the worker
while the caller returned, which the existing Anthropic stream tests reproduce
under Go's race detector. No public API, protocol handling, callback behavior,
or Eino orchestration is changed. Remove this copy when an upstream release
fixes the shared return-variable race and the same tests pass against it.
