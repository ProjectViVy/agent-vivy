# P5.1 diagnostics continuation

## Outcome

Implemented R1/R3 bounded physical-line continuation on `feat/issue32-remediation`.
Code commit: `57bbe2ce3385a7eaf74ebe480bd14babb678b437`.

The reader now uses a strict v2 base64url JSON cursor with file identity,
snapshot size, offset, a SHA-256 anchor over up to 64 preceding bytes, and
physical-line discard state. It validates the opened descriptor against the
resolved path, permits append growth when the anchor matches, and resets with
`Gap=true` for replacement, shrink, or changed bytes before the cursor. Legacy
five-part cursors reset once into v2.

The scan wrapper counts actual bytes returned by the buffered file reader.
Incoming and outgoing anchors share the 4,096,000-byte request ceiling. A page
that stops in an oversized line emits no more than one bounded record and
stores discard state; continuation skips that physical line's remaining bytes.
An incomplete last line remains unparsed at its start offset until a newline
arrives. Final marshaled diagnostic records are valid UTF-8 and at most 8,192
bytes, including escaping and the complete envelope.

## Evidence

- Baseline focused regressions failed in all six observed categories: append
  continuation, same-file truncate/regrow detection, v2 cursor generation,
  incomplete-line handling, final JSON clipping, and legacy reset.
- Linux logging package and race tests passed.
- Actual control RPC read-append-read continuation passed for the GUI log family.
- Windows logging test binary cross-compiles. Native Windows execution remains
  pending.

P5.2 mandatory model settlement remains the next task. Aggregate `just ci` and
product acceptance remain open.
