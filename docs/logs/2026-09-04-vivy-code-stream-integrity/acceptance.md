# Acceptance

1. Stream the reasoning fragments `这`, `是一句`, and `话`; Vivy Code shows
   one continuous reasoning block containing exactly `这是一句话`.
2. Historical or remote `model.delta` events with an empty `delta` do not
   create an empty assistant bubble and do not close the reasoning block.
3. The first non-empty answer delta closes reasoning and starts one normal
   assistant block.
4. Emoji and explicit newlines remain byte-for-byte content, without inserted
   spaces or field-based normalization.
5. A burst exceeding the old 128-event capacity is rendered in order with no
   missing fragments in both built-in and packed TUI paths.
6. If event sequence 3 arrives after sequence 1, the TUI requests replay after
   sequence 1; replayed sequence 2 and 3 are applied once, and a later duplicate
   sequence 3 is ignored.
