# Verification of the baseline repair audit

This delivery changes records only. No repair code, dependency, generated
Assembly, schema, running process or remote branch was changed.

## Scope and source verification

- Compared exact candidate `98b2ea9a` and its executable `397b6f17` with local
  R10/core-fixes, archived donor fixes, provider hardening, issue-94 repairs,
  stable-transfer and CI-contraction branches.
- Used `git for-each-ref`, `git log --all --not 98b2ea9a`, `git show`,
  `git grep`, diff/ancestry checks and current file contents. Missing ancestry
  was not sufficient for a finding; actual surviving code was compared.
- Two read-only reviews independently covered recent repairs/CI and historical
  runtime/session/data donors. The lead also inspected recovery entry points,
  queue admission/drain, session context resolution, SQLite construction,
  provider environment/key handling, discovery and settings reconciliation.
- Inspected the actual pinned Laputa sibling at `9acc9993`, rather than
  assuming a newer-looking commit inherited a divergent Recap repair.
- Checked twenty relevant worktrees. The two dirty historical lanes contain
  no extra unaccounted baseline repair. For `codex/issue91`, both
  `git diff --cached --exit-code f78195a3 --` and
  `git diff --exit-code f78195a3 --` returned zero. The D06 lane's 53 tracked
  changes and untracked Studio format file were compared semantically; its
  untracked file matches committed blob `684f59515b01aa7c38556b28c64c1ef054727e39`.
  No dirty file was changed or removed.

## Current connected-source checks

GitHub issue 83 and issue 94 were read through the connected GitHub tool.
Read-only same-ref comparisons verified the following remote heads on
2026-10-22. They match the local donor evidence used by the audit:

| Remote ref | Verified head |
| --- | --- |
| `remake/core-fixes` | `38fbba6be26eefa8a16a024ec7c98544c49af393` |
| `feat/memory-ci-closure` | `c70adfbe914748edb84eb79bbe5abf6a443a4f7c` |
| `codex/development-ci-contraction` | `40d54eb67f0cfefaf2021782ff5b06a341c2bb73` |
| `main` | `9cd2801f8090723a652d3e835f72d83715b82121` |
| `stable` | `fde3c1f96f086507a35e52689e58fc6e4db981b4` |

A CLI `git ls-remote` attempt could not connect to GitHub from the ordinary
shell. The connected comparison API supplied the head evidence; no remote
update or push was attempted.

## Regression evidence boundary

Historical donor regression names establish what each repair intended to fix;
their earlier results are not current-candidate executions. Representative
fixtures inspected include:

- Operator recovery, session workspace context and literal SQLite path tests.
- Queue storage-failure atomicity, full-options replay/drain and editor restore.
- Ordered notifications with callback RPC and bounded overflow; Work read binding.
- Recoverable patch proposals under approval and auto-approval.
- Native quoted Hook paths, quoted redirects/heredocs and containment.
- Successful Go stderr noise and preserved subprocess causes.
- Environment ambiguity, registry key isolation, default-model reconciliation,
  model discovery bounds and diagnostic append/truncation/replacement.

No product tests, paid provider requests, race, Postgres, full CI or kernel CI
were run for this records-only audit. No freshly reproduced failure is claimed.
Any implementation must run its affected regressions and required current
gates at the resulting candidate. Referenced current source paths and local
audit links were checked; unstaged and staged `git diff --check` passed.

## Human custom-provider report

The owner reported successful use after the credential handoff. This is recorded
as a successful custom-provider manual smoke. The existing Tiny tab still
showed the previous failed smoke and a prepared, unsent workspace prompt when
read; it does not independently certify the complete protected-tool,
follow-up/cancellation/idle checklist. D14's full verdict is not inferred from
the single success report.
