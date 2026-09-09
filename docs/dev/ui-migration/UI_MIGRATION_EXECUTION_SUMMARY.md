# Agent Diva GUI → VIVY UI Full Migration Plan - Execution Summary

## Executive Summary

This document summarizes the completion status of Phase 1 (Infrastructure Preparation) and the plans for subsequent phases.

### ✅ Phase 1 Completion Status (2026-01-XX)

| Task | Status | Deliverable |
|------|------|--------|
| RPC endpoint review | ✅ Complete | `docs/dev/ui-migration/UI_MIGRATION_RPC_GAP_ANALYSIS.md` |
| Internationalization migration plan | ✅ Complete | `docs/dev/ui-migration/UI_MIGRATION_I18N_PLAN.md` |
| Styling system extension plan | ✅ Complete | `docs/dev/ui-migration/UI_MIGRATION_STYLES_PLAN.md` |
| Infrastructure review report | ✅ Complete | `docs/dev/ui-migration/UI_MIGRATION_INFRASTRUCTURE_REVIEW.md` |

**Conclusion for This Phase:**
- VIVY's existing RPC endpoint coverage is **39%** (22/56), with the primary gap in **plan management** (0%)
- Internationalization requires approximately **300 new translation keys** (excluding PET-related keys)
- The styling system requires **10+ CSS variable groups** and **10+ component style files** to be added
- Actual Phase 1 implementation is estimated to require **2-3 weeks**

---

## Key Findings

### 1. RPC Endpoint Gap Analysis

**High-Priority Gaps (Blocker):**
- ❌ `plan/get_active` - Get the active plan
- ❌ `plan/approve` - Approve the plan
- ❌ `plan/reject` - Reject the plan
- ❌ `plan/list_reports` - Get the plan-report list
- ❌ `sessions/generate_title` - Generate a session title automatically

**Impact:** Without these endpoints, the plan-approval workflow cannot be implemented; this is Agent Diva's core differentiating feature.

**Recommended Approaches:** 
- **MVP path:** Implement simplified plan support first (approval decisions only, without version management)
- **Full path:** Port the complete Plan domain model from Agent Diva (2-3 weeks of effort)

### 2. Internationalization Strategy

**Decision:** Adopt a **hybrid namespace** approach
- Retain VIVY's existing flat key names (such as `newSession`)
- Group new features by prefix (such as `chatPlaceholder`, `settingsTitle`)
- Avoid breaking changes while retaining extensibility

**Estimated Effort:** 3.5-4.5 days
- Extend the i18n.ts structure: 1 day
- Update the translate() function: 0.5 day
- Write the completeness-check script: 0.5 day
- Manually review translation quality: 1-2 days
- Add translation-key documentation: 0.5 day

### 3. Styling System Alignment

**Design Principles:** 
- **Retain VIVY's neutral industrial aesthetic** (consistent with the DeepSeek Harness IDE style)
- **Reuse Agent Diva's layout structure and interaction patterns**
- **Adjust colors to match VIVY's palette** (do not use pink gradients)

**Estimated Effort:** 5.5-7.5 days
- Extend tokens.css: 1 day
- Create component style files: 2-3 days
- Import the new style files: 0.5 day
- Remove the Tailwind dependency: 1 day
- Visual regression testing: 1-2 days

---

## Subsequent Phase Planning

### Phase 2: Core Chat System Migration (2-3 Weeks)

**Goal:** Implement a complete conversation interface, including message rendering, streaming output, tool cards, and more.

**Key Tasks:**
1. **Enhance `conversation/view.ts`**
   - Implement a message-list renderer
   - Integrate Markdown rendering (markdown-it + highlight.js)
   - Add the tool-card component (ToolCallCard)
   - Implement collapsible/expandable Thinking blocks

2. **Implement the Composer input area**
   - Text input + submission
   - Plan mode toggle
   - Draft persistence

3. **Integrate the SSE event stream**
   - Reuse the existing `sse.ts`
   - Parse text.delta/tool.start/tool.finish events
   - Manage streaming placeholders (`isStreaming` state)

**Dependencies:** 
- RPC endpoints `turn/start`, `run/subscribe` already exist ✅
- `plan/get_active` needs to be added (if plan mode is supported) ⚠️

**Acceptance Criteria:**
- [ ] Users can send messages and view streaming responses
- [ ] Tool call cards display correctly (name/arguments/result)
- [ ] Thinking blocks can be collapsed/expanded
- [ ] Markdown code blocks are highlighted correctly

---

### Phase 3: Session and Approval Center (1-2 Weeks)

**Goal:** Implement the session sidebar and Approval Center, completing the HITL (Human-in-the-Loop) workflow.

**Key Tasks:**
1. **Enhance `sessions/view.ts`**
   - Render the session list (title/summary/timestamp)
   - Search and filtering
   - Pin/Unpin actions
   - Rename dialog

2. **Implement `reviews/view.ts` (Approval Center)**
   - Paginated approval-list loading
   - Display approval details (tool name/arguments/risk context)
   - Decision buttons (Allow/Deny/Cancel)
   - AskUserQuestion polling timer

**Dependencies:**
- RPC endpoints `session/list`, `session/rename`, `approval/list`, `question/list` already exist ✅
- `sessions/generate_title`, `sessions/pin` need to be added ⚠️

**Acceptance Criteria:**
- [ ] Users can create/rename/delete sessions
- [ ] The Approval Center displays pending approval items
- [ ] Users can approve/reject approvals
- [ ] AskUserQuestion polls and displays correctly

---

### Phase 4: Settings Panel (2-3 Weeks)

**Goal:** Implement a complete settings interface, including Provider/Channel/Skills/MCP management.

**Key Tasks:**
1. **Significantly extend `settings/view.ts`**
   - Provider management (built-in + custom)
   - Channel management (Telegram/Discord/QQ, etc.)
   - Skills marketplace browsing and installation
   - MCP server management
   - Audit-log viewing
   - Theme/language switching

2. **Implement form controls**
   - Input/Select/Checkbox
   - Step-by-step Wizard form
   - Validation and error messages

**Dependencies:**
- RPC endpoints `settings/get`, `settings/update` already exist ✅
- `providers/list`, `channels/list`, `skills/list`, and others need to be added ⚠️

**Acceptance Criteria:**
- [ ] Users can add/edit/delete Providers
- [ ] Users can configure Channels
- [ ] Users can browse and install Skills
- [ ] Theme and language switching works correctly

---

### Phase 5: Memory and Advanced Features (2-3 Weeks)

**Goal:** Implement memory management, the Persona workspace, and Evolution proposal review.

**Key Tasks:**
1. **Create `memory/view.ts`**
   - Browse BML memories (short-term/long-term/core)
   - FTS5 full-text search
   - Edit/delete memory entries

2. **Implement the Persona Markdown editor**
   - Integrate CodeMirror
   - Lock the Frozen Core
   - Version history

3. **Implement Evolution proposal review**
   - Change suggestions generated by AutoDream
   - Governed apply (apply after review)
   - Rollback support

**Dependencies:**
- New endpoints such as `memories/list`, `persona/get`, and `evolution/list_proposals` are required ❌

**Acceptance Criteria:**
- [ ] Users can browse and search memories
- [ ] Users can edit Persona Markdown
- [ ] Users can review and apply Evolution proposals

---

### Phase 6: Console and Diagnostics (1 Week)

**Goal:** Implement Gateway status monitoring, a log viewer, and Token statistics.

**Key Tasks:**
1. **Create `console/view.ts`**
   - Gateway health checks
   - Channel connection status
   - Cron task list

2. **Implement the log viewer**
   - Real-time log stream (SSE)
   - Filtering/search
   - Level switching (info/debug/error)

3. **Add the Token statistics panel**
   - Session-level token usage
   - Budget-threshold warnings

**Dependencies:**
- New endpoints such as `stats/tokens` and `audit/log` are required ❌

**Acceptance Criteria:**
- [ ] Users can view Gateway status
- [ ] Users can view real-time logs
- [ ] Users can view Token statistics

---

### Phase 7: Onboarding and Wrap-up (1 Week)

**Goal:** Implement the welcome wizard and complete error handling and boundary-case handling.

**Key Tasks:**
1. **Create `onboarding/view.ts`**
   - DeepSeek API Key input
   - Bocha search key configuration
   - Quick navigation (chat/providers/network/console)

2. **Complete error handling**
   - Retry network timeouts
   - RPC error messages
   - Boundary-case handling

3. **Optimize performance**
   - Virtual scrolling (long message lists)
   - Lazy loading (render the settings panel on demand)

**Acceptance Criteria:**
- [ ] The welcome wizard appears on first launch
- [ ] All errors have user-friendly messages
- [ ] Long lists scroll smoothly (60fps)

---

### Phase 8: Testing and Release (1-2 Weeks)

**Goal:** Complete comprehensive testing and publish a release candidate.

**Key Tasks:**
1. **Unit Tests**
   - Migrate Agent Diva's Vitest tests
   - Cover key utility functions (Markdown rendering, approval idempotency, and more)

2. **E2E Tests**
   - Extend the existing Playwright configuration
   - Write core-path tests:
     - Create session → send message → view response
     - Plan-approval workflow
     - Settings-modification workflow

3. **Visual Regression Tests**
   - Compare screenshots of key pages
   - Ensure Light/Dark modes work correctly

4. **Performance Benchmarking**
   - First-screen load time ≤ 2s
   - Lighthouse score ≥ 90
   - Build output size ≤ 500KB (gzipped)

**Acceptance Criteria:**
- [ ] E2E tests for all core paths pass
- [ ] No console error/warning
- [ ] Lighthouse performance score ≥ 90
- [ ] Build output size ≤ 500KB (gzipped)
- [ ] User acceptance testing passes

---

## Overall Schedule

| Phase | Duration | Start Date | End Date |
|------|------|---------|---------|
| Phase 1: Infrastructure Preparation | 2-3 weeks | 2026-01-XX | 2026-02-XX |
| Phase 2: Core Chat System | 2-3 weeks | 2026-02-XX | 2026-03-XX |
| Phase 3: Sessions and Approval | 1-2 weeks | 2026-03-XX | 2026-03-XX |
| Phase 4: Settings Panel | 2-3 weeks | 2026-03-XX | 2026-04-XX |
| Phase 5: Memory and Advanced Features | 2-3 weeks | 2026-04-XX | 2026-05-XX |
| Phase 6: Console and Diagnostics | 1 week | 2026-05-XX | 2026-05-XX |
| Phase 7: Onboarding and Wrap-up | 1 week | 2026-05-XX | 2026-05-XX |
| Phase 8: Testing and Release | 1-2 weeks | 2026-05-XX | 2026-06-XX |
| **Total** | **12-18 weeks** | | |

**Note:** The schedule above is a serial estimate; some tasks can run in parallel during execution (for example, internationalization migration can run alongside styling extensions).

---

## Resource Requirements

### Staffing
- **Frontend Developer:** 2 people (full-time)
- **Backend Developer:** 1 person (part-time, responsible for adding RPC endpoints)
- **QA Engineer:** 1 person (part-time, full-time during Phase 8)
- **Product Manager:** 1 person (part-time, responsible for translation review and visual acceptance)

### Technical Dependencies
- **Node.js**: >= 18 (required by Vite)
- **TypeScript**：>= 5.0
- **Playwright**：>= 1.40
- **Go**: >= 1.26 (backend development)

---

## Risk Overview

### High Risk
1. **Designing the Plan domain model from scratch**
   - Impact: May delay Phase 2
   - Mitigation: Use the MVP path and implement a simplified version first

2. **RPC endpoint additions take more effort than expected**
   - Impact: Blocks frontend development
   - Mitigation: Involve Backend early and complete high-priority endpoints during Phase 1

### Medium Risk
3. **Styling migration requires substantial effort**
   - Impact: Inconsistent UI visuals
   - Mitigation: Follow VIVY design tokens strictly; do not introduce creative deviations

4. **Internationalization key-name conflicts**
   - Impact: Confusing translations
   - Mitigation: Use namespace prefixes and run the completeness-check script

### Low Risk
5. **Residual Tailwind dependency**
   - Impact: Larger build output
   - Mitigation: Use grep to confirm that no Tailwind class names remain

---

## Next Actions

### Start Immediately (This Week)
1. **Backend Team:** Implement the high-priority RPC endpoints
   - `plan/get_active`
   - `plan/approve`
   - `plan/reject`
   - `sessions/generate_title`

2. **Frontend Team:** Begin actual Phase 1 implementation
   - Extend `i18n.ts` (according to `UI_MIGRATION_I18N_PLAN.md`)
   - Extend `tokens.css` (according to `UI_MIGRATION_STYLES_PLAN.md`)

3. **PM:** Organize a review meeting
   - Review this plan document
   - Confirm priorities and the schedule
   - Allocate resources

### Goals for Next Week
- Complete the Phase 1 infrastructure implementation
- Begin development of the Phase 2 core chat system
- Backend completes the high-priority RPC endpoints

---

## Appendix: Reference Documents

1. **RPC Endpoint Gap Analysis:** `docs/dev/ui-migration/UI_MIGRATION_RPC_GAP_ANALYSIS.md`
2. **Internationalization Migration Plan:** `docs/dev/ui-migration/UI_MIGRATION_I18N_PLAN.md`
3. **Styling System Extension Plan:** `docs/dev/ui-migration/UI_MIGRATION_STYLES_PLAN.md`
4. **Infrastructure Review Report:** `docs/dev/ui-migration/UI_MIGRATION_INFRASTRUCTURE_REVIEW.md`
5. **Original Migration Plan:** (see the complete plan when exiting plan mode)

---

**Document Version:** v0.1  
**Created:** 2026-01-XX  
**Last Updated:** 2026-01-XX  
**Maintainer:** UI Migration Team  
**Status:** ✅ Phase 1 planning complete, pending implementation
