# Plugin v1 compiler acceptance fixtures

This corpus is pure input data for the future P1 Assembly Compiler. It is
deliberately separate from the active Web/TUI I18N paths and from the legacy
v0 fixtures that P1 will replace.

`cases.json` is the stable index. Each case records whether compilation should
accept or reject the Recipe and descriptors. Rejected cases include the
normative rule and a stable diagnostic substring. P1 tests should load this
corpus through the real compiler; `scripts/check-plugin-v1-fixtures.mjs` only
checks corpus integrity before that compiler exists.
