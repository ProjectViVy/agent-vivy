# Verification

- `just ci`: blocked, exit 127: `just: command not found` in this fresh checkout environment. CI is not reported as passing.
- `git diff --check`: passed after edits.
- Markdown structure and local relative-link checks: passed.
- Runtime, browser, external memory, and device tests: not run; this delivery changes documentation only. SCX examples remain reference specifications, not executed tests.
