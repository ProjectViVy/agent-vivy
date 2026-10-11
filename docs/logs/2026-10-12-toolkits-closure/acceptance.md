# Toolkits acceptance reconciliation — 2026-10-12

This is an evidence checkpoint for TK-13. “Proven” means the stated slice has a passing test or artifact observation; it does not imply that the entire broader acceptance row is closed. The Epic remains In progress until every B1–B14 requirement and A01–A28 case has complete evidence.

## Requirement status

| Requirement | Current evidence | Remaining acceptance |
| --- | --- | --- |
| B1 Existing owners interpret content | Real App/UI reads a captured Skill; live `server-everything` stdio smoke completes a scripted run through MCPHost and ToolHost, including approval, result journaling and shutdown. `TestToolkitMCPConsentCaptureGovernsRealServiceRun` proves a disabled installed Toolkit MCP declaration can be consented, enabled, captured and invoked through Runtime Service with Journal and approval evidence. | Complete Rule/Hook governed runs and the broader MCP unreachable/schema/revocation matrix. |
| B2 Multiple Masks and role/procedure separation | PM and Dev Masks are present in native UI E2E; recommendations stay distinct from Skills. | Full selection/run snapshot and update race matrix. |
| B3 Recommendation does not grant or activate | Real UI demonstrates independent Skill use while PM/Dev recommendations are shown. | Broader before/after model, grant and consent invariance evidence. |
| B4 Unambiguous identity | Identity, digest, source and storage tests pass; artifact UI exposes release digest. | Complete cross-source/reinstall collision integration cases. |
| B5 One captured epoch | Runtime root/report/workflow snapshot and revocation tests pass; browser observes catalog epoch increments. The real Toolkit MCP Service test records the exact consent digest/revision in its captured run snapshot. | Complete all admission branches, checkpoint/child/queue resume and concurrency tests. |
| B6 Installation does not execute content | Browser proves MCP is not eagerly connected during install; source acquisition validates declared content. | Explicit process/network spies for all component kinds and update/failure cases. |
| B7 Separate exact Hook consent | Browser confirms Hook consent separately and requires new consent after declaration update. | Deny/rewrite/crash/timeout/disarm protocol matrix through the live App. |
| B8 Invalid update preserves prior state | Update preparation and stale-source/rebind tests pass; browser observes explicit update publication and fresh consent. | Crash and CAS failure injection across every publication boundary. |
| B9 Revocation narrows live use | Focused Runtime and storage fence tests pass; UI disable/revoke operations work. | Pending cancellation and every live-call boundary under concurrent runs. |
| B10 Recovery preserves evidence | App restart preserves uninstall history; storage suites pass on SQLite and PostgreSQL. | Crash-after-commit/pointer recovery, snapshot compatibility and reference-retention matrix. |
| B11 Missing owner is visible/rejected | Omission artifacts remove owners; App reports Mask/MCP unavailable and rejects previews; UI/action omission is inspected. | User-facing error-state coverage and all owner/kind combinations. |
| B12 Content cannot expand Generation/Grant graph | Three inspected artifacts keep protected Tool exports unchanged; omitted Modules/ToolWorlds stay omitted. | Complete runtime grant and frozen-generation invariance assertions for lifecycle changes. |
| B13 Dedicated Toolkit UI | Desktop lifecycle E2E passes; 390px E2E covers keyboard navigation, populated Skill details, preview failure/success, long content, dialog bounds, Escape and focus restoration. Full UI suite passes. | Broader viewport widths, blocker states and remaining operation failure/recovery states. |
| B14 Native-only install | Local native fixture passes real preview/install; strict contract/parser tests pass. | Full foreign-format and adversarial source matrix through the user-facing action. |

## Failure-case checkpoint

The packaged E2E covers the main path for A01, A07, A09, A13, A15 and A23: native preview/install, captured Skill read, recommendation independence, exact Hook consent refresh, MCP binding/consent without eager connection, and uninstall/orphan history. `TestToolkitMCPConsentCaptureGovernsRealServiceRun` adds the authorized MCP call and exact consent snapshot path for A15. App authority tests cover the relevant A24 direct-control boundary. Artifact inspection and owner omission tests cover the central A26 omission behavior.

Focused unit and backend tests also cover many isolated conditions from A03–A16, A21–A22 and A25, as recorded in the prior story evidence and current verification log. The following integration groups remain open: source-change/reinstall and every publication crash point (A02, A06, A17–A20); the complete run-admission and snapshot restore matrix (A05, A08, A10–A12, A16, A19, A21–A22); UI stale/lost-response/error recovery beyond the tested cases (A25); broader viewport and blocker coverage (A27); and fresh/upgrade/fault rollback compatibility across both databases (A28).

The status above is deliberately partial. Full `just ci` could not run because the host lacks `just`, `pwsh` and `powershell`. PostgreSQL itself is no longer blocked: live disposable PostgreSQL conformance passed. TK-13 remains In progress pending remaining evidence and review.
