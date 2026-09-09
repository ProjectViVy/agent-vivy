# Verification — reference-project license review

Date: 2026-09-02.

```text
ls .workspace/
  → eight directories (agent-wiki-library caveman crush deepseek-harness eino headroom oh-dsh smoke);
    confirmed no local claude-code / rig copies → review switched to upstream evidence

curl -sL -H "Accept: application/vnd.github.raw+json" \
  https://api.github.com/repos/anthropics/claude-code/contents/LICENSE.md
  → one line in full: "© Anthropic PBC. All rights reserved. Use is subject to
    Anthropic's Commercial Terms of Service." (P3-1 evidence)

curl -sL https://api.github.com/repos/anthropics/claude-code → license: None

curl -sL https://raw.githubusercontent.com/0xPlaygrounds/rig/main/LICENSE
  → standard MIT in full, copyright line "Copyright (c) 2024, Playgrounds Analytics Inc."
    (P3-2 evidence; the body is the MIT terms verbatim, with no additional restrictions)

just ci   (combined docs gate with today's SR-4 / P2-1 slices)
  → CI-EXIT:0
```

Note: a direct fetch of `anthropics/claude-code/main/LICENSE.md` from
raw.githubusercontent.com returned empty (branch/path probing difference), so the GitHub
contents API was used to obtain the same content; the two sources corroborate each other.
