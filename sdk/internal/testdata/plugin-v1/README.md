# Plugin v1 compiler acceptance fixtures

This corpus is input data for the P1 Assembly Compiler. It is deliberately
separate from the active Web/TUI I18N paths and contains both accepted v1
examples and explicit legacy-rejection cases.

`cases.json` is the stable index. Each case records whether compilation should
accept or reject the Recipe and descriptors. Rejected cases include the
normative rule and a stable diagnostic substring. Compiler tests load this
corpus through the real implementation; `scripts/check-plugin-v1-fixtures.mjs`
also checks its structural integrity.
