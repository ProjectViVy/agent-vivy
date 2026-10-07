# Verification

## Sources and review

Inspected both complete designs, both indexes and every earlier Story's
scope/tasks. Source `99f9b7b2` changes exactly ten planning files. The current
index maps all ten to successor material, while design 1.1 records deliberate
alternatives/superseded decisions. Checked the conditional replay contract
against the earlier cursor, terminal, persistence, error and artifact cases.
The newer native/SDK baseline is retained without merging old runtime code.

Applied existing supermanagement, writing-plans and verification guidance.
Reviewed the changed design and plans directly; no parallel implementation or
new product scope was authorized. Scope disagreements remain in G0.

## Checks

- `git diff --check`: passed.
- Focused Python document validation: 13 design/plan/log documents have
  balanced fences; 63 relative links/anchors (including the TODO entry)
  resolve; all ten earlier source documents have successor mappings.
- Eight Story IDs and 20 task IDs are unique. All 100 numbered task steps
  are ordered 1–5. Ten requirements have Story owners; all 15 base design
  fixtures remain mapped, with conditional replay cases added.
- Parsed the index dependency table: no missing predecessor, self-edge or
  cycle. The base path remains A2A-00 through A2A-06; B adds only A2A-R1
  after A2A-06. Reviewed and removed a potential acceptance-cycle wording:
  A2A-06 accepts the standard milestone without waiting for downstream B.
- Exactly nine intended Markdown files changed; runtime/source/config and
  the old branch working tree are untouched.
- Publication uses the newer head as first parent and `99f9b7b2` as second
  parent. Compare the remote tree to the locally staged tree and verify
  both parents/human attribution before moving the `A2A` branch ref.

## Unavailable product gate

`just ci` was attempted from the repository root and exited 127:
`just: command not found`. No CI pass is claimed. This delivery changes only
Markdown planning documents; no SDK probe, proposed Story test, real client,
SQLite/PostgreSQL suite, pack/Inspect or runtime smoke ran. Functional gates
and the conditional extension remain unaccepted.
