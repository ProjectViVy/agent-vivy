# Acceptance

1. In a VIVY CODE session backed by a thinking-capable model, the footer and
   right rail show the draft thinking state and `Ctrl+T` cycles
   `auto -> on -> off -> auto`.
2. `/thinking on`, `/thinking off`, and `/thinking auto` update the same state;
   `/status` labels it as the next-turn preference.
3. Sending a turn transmits that state through `turn/start.thinking`.
4. Turns queued under different states retain the state selected when each was
   queued.
5. For a model without authoritative thinking support, the shortcut is hidden
   and forcing `/thinking on` produces a local error instead of a fake mode.
6. Creating or selecting a new session refreshes model capability; moving from
   a supported model to an unsupported one cannot leave forced thinking on.

The selector controls runtime extended thinking only. It does not claim to
change the provider/model or reasoning effort beyond the existing
`auto/on/off` contract.
