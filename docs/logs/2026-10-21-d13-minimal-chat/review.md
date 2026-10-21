# Independent review

Fresh read-only reviewer: d13_final_review. The reviewer inspected the D13 diff,
applicable instructions, Recipe/source/Port ownership, examples, UI state and
privacy, and the actual pinned Eino execution boundary. It did not edit source
or certify test execution.

Three P2 findings were substantiated and corrected:

1. Live TUI creates an unbound Session by default, so the seeded example file
   would not be in the Run workspace. Both examples now invoke the focused
   bind-workspace command through existing tui.Dial/session-create and select
   its confirmed Session. Actual packed tests execute that command and verify
   the returned workspace before a code-face causal Run. hello_stat is also
   explicitly included in the normal enabled-tool ceiling.
2. Async admission could clear text edited while waiting. The textarea is now
   disabled only during admission. A delayed-admission regression first failed,
   then passed; failure retains the draft.
3. Tool result text could hide a simultaneous durable failure, and empty
   completed output looked Running. Projection now tracks finished state and
   renders result/failure independently. The representative regression first
   failed and passes in the final component suite.

Correction re-review found no unresolved substantiated issue. The helper has no
Run/provider/model/transport/listener ownership. The UI consumes the exclusive
std/ui-root through PresentationHost and FullUIHost/FaceClientStore. The existing
v0.9.21 ADK typed agent/runner, ToolsNode and WrapModel remain sole execution
owners. D14 retains independent final integrated reviewers and release gates.
