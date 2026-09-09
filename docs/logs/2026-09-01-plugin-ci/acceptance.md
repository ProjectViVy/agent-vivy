# Acceptance

Manual verification (without reading code):

1. From the repository root, run `just plugin-ci`: the output has six
   `== plugin-ci: <name>` sections (dingtalk/discord/feishu/lsp/qq/telegram),
   with vet + test passing in each and exit code 0.
2. From the repository root, run `just ci`: a plugin-ci section appears between
   headless-compile and ui-ci.
3. Reverse verification: introduce a vet/test failure in any plugin module (for
   example, add a dependency that has not been tidied); `just ci` fails as a
   whole, proving that plugin regressions are no longer checked only manually
   within an individual slice.
4. `plugins/qq` passes vet/test on a clean checkout without any manual steps
   (the drift has been fixed).
