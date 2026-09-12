# PLG-P3 closure acceptance

PLG-P3 is accepted when the following observable statements remain true:

- A protected Tool, public static Tool, dynamic ToolWorld Tool, and MCP-derived
  Tool all traverse the same governed runtime envelope.
- A pre-tool Middleware rewrite is represented in the approval request after
  Secret redaction; the original arguments cannot be presented for approval
  while different rewritten arguments reach the Provider.
- Approval authority is bound to the Tool identity and canonical
  post-Middleware arguments; a different resume-time rewrite marks the
  approval stale and never invokes the Provider.
- The binding guard runs at the sole Provider dispatch seam, so a replayed
  allow/pass/auto path and a legacy approval without a digest cannot bypass it.
- Approval projections redact Secrets introduced by Middleware while protected
  checkpoint state retains the exact arguments needed for verification.
- No effectful Provider runs before the durable approval decision.
- Policy and approval events precede Tool start and finish events in the
  Journal for every source class.
- Tool output is marked untrusted, bounded, and Secret-redacted before either
  Journal or Observer projection.
- Protected Tool identities remain reserved even when their default Provider
  is omitted.
- Observer delivery cannot mutate Tool results, and status reads remain
  bounded and lifecycle-free.

Executable acceptance is
`TestToolEnvelopeConformanceAcrossAllSourceClasses` together with the focused
ToolHost, ModuleHost, ObserverHost, StatusHost, Runtime, App, and Assembly
suites recorded in `verification.md`.
