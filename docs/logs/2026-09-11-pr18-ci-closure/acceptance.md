# Acceptance

The PR 18 CI closure is accepted when all of the following are true:

1. The cross-face i18n validator reports no unclassified MCP resource-bridge or capability-state keys.
2. The plugin import-boundary test scans the v1 public SDK surfaces and does not reference the removed v0 `sdk/plugin` directory.
3. The approval-resume regression can run repeatedly without deciding an approval before its Journal event is durable.
4. Equivalent LF and CRLF NUL-free UTF-8 source trees produce the same Module digest, while invalid UTF-8 or NUL-containing binary files remain byte-exact.
5. `vivy-sdk verify` accepts the DingTalk Module with its canonical `249983bd...` source pin on Windows.
6. Recipe pack and artifact inspection complete successfully on Windows without relying on POSIX executable mode bits.
7. The complete `just ci` product gate passes within the declared 20-minute Go package timeout.

There is no visible UI acceptance step because this delivery does not alter rendered UI or interaction behavior.
