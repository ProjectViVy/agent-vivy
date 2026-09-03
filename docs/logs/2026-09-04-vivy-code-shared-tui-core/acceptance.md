# Acceptance

1. Build/test either TUI module and inspect that `internal/tui/events.go` and
   `faces/tui/events.go` contain only wire-type adapters; event interpretation
   is implemented once in `sdk/tui/stream`.
2. Deliver reasoning chunks such as `这`, `是一句`, and `话🙂\n继续`, with
   empty answer deltas interleaved. Both faces keep one continuous reasoning
   block and begin the answer block only on a non-empty answer delta.
3. Deliver an unknown durable event followed by a numbered model delta. The
   stream cursor advances across the unknown event; a duplicate numbered
   event is ignored and a missing sequence requests replay from the last
   contiguous cursor.
4. Deliver a burst larger than the old bounded channel capacity. All notices
   arrive in order without a dropped delta in both built-in and packed faces.
