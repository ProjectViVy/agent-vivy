# Acceptance

- When a successful Generate response cannot be settled, its caller receives a
  settlement error and no successful response.
- When a provider or Chunk operation fails and settlement also fails, callers
  can identify both causes with `errors.Is`.
- A completed stream delivers provider chunks, then the settlement error, then
  EOF. End runs once before the terminal error is observable; its input retains
  the provider's usage and response-complete evidence.
- Closing the consumer while End is in progress does not leave the pump blocked
  trying to deliver its settlement error; the upstream producer is released.
- Normal successful calls, Begin fail-closed behavior, binding scope, and
  producer backpressure continue to pass their existing regression tests.

The deterministic runtime model tests exercise the actual observing wrappers
and Eino pipes with an observer whose settlement persistence is forced to fail.
