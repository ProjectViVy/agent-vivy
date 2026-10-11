# Toolkit acceptance and evidence matrix

**Revision:** TK-P1. **Status:** Requirements and exit criteria; observed implementation evidence and open gates are recorded in the [TK-13 closure log](../../../logs/2026-10-12-toolkits-closure/acceptance.md).\
**Authority:** [architecture](../../../architecture/VIVY-CONTENT-BUNDLES.md), [TK-C1](contracts.md), and [index status/DAG](index.md).

## Requirement ownership

| Requirement | Owning implementation Stories | Integrated proof |
| --- | --- | --- |
| B1 Existing owners interpret all content | TK-04, TK-06–10 | Real app exposes content through existing Skill/Mask/Rules/Hook/MCP owners; TK-13 |
| B2 Multiple Masks and role/procedure separation | TK-01, TK-07, TK-11 | Native PM/Dev definitions plus independent PRD/implementation Skills |
| B3 Recommendations do not grant or activate | TK-06–07, TK-12 | Before/after selection inventory, model, grants and consent are unchanged |
| B4 Unambiguous identity | TK-01–03, TK-06, TK-09 | Cross-source collision, install/reinstall and stable child-ID matrix |
| B5 One captured epoch | TK-03–09 | All admission branches, paused/child runs, concurrent settings and update |
| B6 Install/prepare execute no toolkit content | TK-01–02, TK-04, TK-08–10 | Process/network spies for payload execution, separate from explicit source acquisition |
| B7 Separate exact Hook consent | TK-08, TK-10, TK-12 | Inert until arm, changed digest requires arm again, revoked guard fails |
| B8 Invalid update preserves old state | TK-02–05, TK-09–12 | CAS/prepare failures and crash points preserve committed identity |
| B9 Revocation narrows live use | TK-03–05, TK-07–10, TK-12 | Next boundary denied; pending cancellation does not undo fence |
| B10 Recovery preserves evidence | TK-02–05, TK-07 | Restart, schema compatibility, retention and exact snapshot recovery |
| B11 Unsupported/missing owner is visible | TK-01–02, TK-04, TK-06, TK-08–12 | No silently dropped component; missing-owner UI and native validation errors |
| B12 No Generation/Grant expansion by content | TK-01, TK-04–06, TK-08–10 | Install does not mutate frozen Module graph or grants; omitted owners remain omitted |
| B13 Dedicated Toolkit UI | TK-10–12 | Real route/navigation, native installation, all components and complete operation states |
| B14 Native-only installation | TK-01–02, TK-10–11 | Foreign-only source rejected; no importer/converter/format detection |

TK-13 owns the final combined proof for every row; it does not implement missing predecessors by bypassing their owners.

## Failure and user-flow cases

| Case | Setup and action | Required observation | Primary Story |
| --- | --- | --- | --- |
| A01 | Preview/install PM+Dev native local toolkit | Exact source/digest/components shown; disabled install; no payload execution | TK-02, TK-10–11 |
| A02 | Source changes after preview | Install uses retained reviewed bytes or rejects stale preview; no refetch substitution | TK-04, TK-10 |
| A03 | Upstream-only plugin manifest | unsupported_format; no conversion attempt | TK-01–02 |
| A04 | Duplicate key/unknown field/event/authority frontmatter | Whole candidate fails static validation | TK-01 |
| A05 | Path/link/case collision/limit attack | No read/write outside staged root; existing object unchanged | TK-02 |
| A06 | Same BundleID from another source/reinstall | Conflict or explicit reviewed rebind; old InstallID/consent never inherited | TK-03–04 |
| A07 | Use toolkit Skill and declared text resource | Exact captured content through SkillHost/skill_view; no filesystem mount grant | TK-06 |
| A08 | Attempt resource traversal/binary read/Skill mutation | Typed rejection before write; local Skills unaffected | TK-06 |
| A09 | Select PM then independently invoke implementation Skill | PRD recommendation shown; independent Skill remains available under ordinary policy | TK-07, TK-12 |
| A10 | Change Mask during run/copy Mask/update bundle | Current run retains snapshot; next run captures new selection; custom copy unchanged | TK-07 |
| A11 | Remove selected Mask/disable toolkit/uninstall | Removal/uninstall blocked by references; disable leaves unavailable selection; never silently unmask | TK-07, TK-12 |
| A12 | Rule @import or prompt overflow | Admission fails before model request; no silent rule truncation | TK-08 |
| A13 | Unarmed/changed Hook; then exact arm | Initially zero calls, exact arm enables, changed release invalidates consent | TK-08, TK-12 |
| A14 | Hook deny/rewrite/crash/timeout/disarm | Existing policy recheck/denial applies; disarm cannot bypass captured guard | TK-08 |
| A15 | MCP missing binding/consent/unreachable | Inactive/unavailable distinguishable; no eager connection from install | TK-09, TK-12 |
| A16 | MCP source collision/schema change/revoke | Candidate conflict or stale identity error; no config substitution/reopen after fence | TK-09 |
| A17 | Update during approval wait/checkpoint/child work | Existing epoch stable; blockers visible; new roots wait; existing child/resume can finish | TK-05, TK-12 |
| A18 | Concurrent settings/update/Mask selection | One CAS-consistent outcome; no mixed owner view | TK-04–05, TK-07, TK-09 |
| A19 | Ordinary/continuity/goal/queue/child/report/edit new run | Prompt/bundle/start evidence atomic in every branch | TK-05 |
| A20 | Crash before/after epoch commit and pointer swap | Recover exact committed old/new epoch before opening admission | TK-03–05 |
| A21 | Missing/corrupt artifact at resume | Explicit snapshot/recovery error, never latest fallback | TK-02, TK-05 |
| A22 | Revoke during dispatched tool call | No claim of undo; next governed boundary rejects; explicit cancel uses existing controls | TK-05 |
| A23 | Uninstall, source orphaning and history retention | User files/credentials/custom copies remain; referenced objects survive; orphan usable | TK-04 |
| A24 | Anonymous/public-owner/agent-bound consent action | Auth/owner/policy denies; no human authorization inferred from bundle text | TK-10 |
| A25 | UI stale response/lost write reply/reload/conflict | Request identity guard; same operation receipt recovered; dirty input preserved | TK-11–12 |
| A26 | Default and omitted UI/Mask/MCP Generations | Assembly controls page and capability presence; no dynamic Module/Port loading | TK-10–13 |
| A27 | Toolkit pages at desktop/mobile, keyboard navigation | Usable localized 工具包 entry, component detail, errors, consent and blockers | TK-11–13 |
| A28 | Fresh/upgrade/reopen/fault rollback on both DBs | Same logical constraints/receipts/CAS and old snapshot compatibility | TK-03, TK-07, TK-13 |

## Evidence package and exit rules

Every implemented Story returns its commit/source identity, exact commands, environment, fixtures, observed outputs, unresolved failures and applicable iteration log. Store evidence links in the index; do not copy “passed” from another Story's plan. The execution lane must have authorization to run implementation tests. The default full gate is `just ci`; PostgreSQL requires non-skipped disposable DSN-backed conformance. Real browser smoke uses the split Vite server; artifact pack/inspect supplements it.

The baseline's current Playwright configuration starts an embedded-backend setup and uses a Windows Go default. TK-13 provides an explicit split-toolkit configuration rather than silently treating the old venue as sufficient. Existing unrelated failures must be reproduced and attributed; record their effect on acceptance without claiming this feature fixed them.

An Epic exits only when its combined outcome is proven. Full v1 requires all B1–B14 rows and A01–A28 cases accepted, all five kinds available through their owners, the dedicated UI, and final review of real integration evidence. Missing environment access leaves a gate blocked. Publication/deployment and external tracker writes require their own existing authorization.
