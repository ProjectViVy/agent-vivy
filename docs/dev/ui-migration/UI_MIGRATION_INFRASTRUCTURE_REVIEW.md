# UI Migration Infrastructure Review Report

## Executive Summary

The comprehensive review of the VIVY Go backend RPC endpoints is complete. The conclusions are as follows:

### ✅ Capabilities Already Available
1. **Complete session management** (CRUD + message list)
2. **Run control and event streams** (SSE subscription, cancellation, log replay)
3. **Approval and question system** (tool-level HITL)
4. **Review Center** (unified Approval/Question view)
5. **Subprocess management** (start/query/wait/cancel)
6. **Studio artifact management** (Generations/Evals/Promotions)
7. **Basic settings management** (Provider/Model/BaseURL)

### ⚠️ Capabilities to Add
1. **Plan-management endpoints** (5 new endpoints) - **High priority**
2. **Extended configuration management** (Provider CRUD + testing) - **Medium priority**
3. **Channel-management endpoints** (confirm whether an implementation already exists) - **Medium priority**
4. **Skills-management endpoints** - **Low priority**
5. **Memory and Persona endpoints** - **Low priority**
6. **Audit and diagnostics endpoints** - **Low priority**

### 📊 Domain-Model Status
- ✅ **Todo** - `domain.Todo` and `storage.TodoStore` already exist
- ⚠️ **Plan** - Only the `PolicyProfilePlan` enum value exists; there is no complete domain model
- ❌ **PlanReport** - Completely missing
- ⚠️ **Approval** - Exists but may not support `domain='plan'`

---

## Detailed Analysis

### 1. RPC Endpoint Coverage

| Function Category | Agent Diva Requirements | Existing in VIVY | Coverage | Gap |
|---------|----------------|----------|--------|------|
| Session management | 6 | 6 | 100% | 0 |
| Run control | 6 | 6 | 100% | 0 |
| Approval and questions | 4 | 4 | 100% | 0 |
| Review Center | 3 | 3 | 100% | 0 |
| **Plan management** | **5** | **0** | **0%** | **5** |
| Configuration management | 6 | 2 | 33% | 4 |
| Channel management | 5 | 0? | 0%? | 5? |
| Skills management | 4 | 0 | 0% | 4 |
| Memory management | 8 | 0 | 0% | 8 |
| Audit and diagnostics | 4 | 1 | 25% | 3 |
| Other helpers | 5 | 0 | 0% | 5 |
| **Total** | **56** | **22** | **39%** | **34** |

### 2. Detailed Key Gaps

#### 2.1 Plan Management (Blocker)

**Reason for Absence:** VIVY's PRD v0.5 focuses on the core Agent Loop; plan functionality is an Agent Diva Pro feature and has not yet been ported to VIVY.

**Impact:**
- Cannot display plan cards pending approval
- Cannot approve/reject plans
- Cannot view plan-execution progress
- Cannot return plans to draft

**Solution Options:**

**Option A: Port the Complete Plan Domain Model from Agent Diva**
- Pros: Complete functionality and a consistent user experience
- Cons: Large effort (requires designing the domain/storage/runtime layers)
- Estimated effort: 2-3 weeks

**Option B: Simplified Plan Support (Approval Decisions Only)**
- Pros: Quickly implements the core path
- Cons: Lacks advanced functionality such as plan-detail display and version management
- Estimated effort: 3-5 days

**Recommendation: Option B (MVP) → Iterate toward Option A later**

#### 2.2 Configuration Management (Important but Not a Blocker)

**Current State:** VIVY has `settings/get` and `settings/update`, but they support only Provider/Model/BaseURL.

**Fields to Extend:**
```typescript
interface RuntimeConfig {
  // existing
  provider: string;
  default_model: string;
  base_url: string;
  
  // to add
  api_key?: string;  // ⚠️ Note: secrets must not be persisted
  
  // provider list
  providers: Record<string, ProviderConfig>;
  custom_providers: Record<string, ProviderConfig>;
  
  // tool configuration
  tools: ToolsConfig;
  
  // budget and compaction
  budget: BudgetConfig;
  
  // other
  sandbox_policy: SandboxPolicy;
  compaction_threshold: number;
}
```

**Note:** VIVY's design principle is "Secrets are never persisted" (D-010), so sensitive information such as API keys cannot be managed through the settings API. A secure key-input flow is needed (stored in memory and cleared on restart).

#### 2.3 Channel Management (Confirmation Required)

**Question:** No channel-related case was found in `control.go`, but the README describes VIVY as a "personal gateway Agent", which should theoretically support multiple channels.

**Action Items:**
1. Check whether `internal/app/` contains a channel manager
2. Check whether the entry point in `cmd/vivy/` initializes channels
3. If they are indeed missing, design a Channel Store and RPC endpoints

---

## Implementation Roadmap

### Phase 1: Infrastructure Preparation (Current Phase)

#### 1.1 RPC Endpoint Additions (1-2 Weeks)
- [ ] **Plan-management MVP** (3 endpoints)
  - `plan/get_active` - Get the active plan
  - `plan/approve` - Approve the plan
  - `plan/reject` - Reject the plan
  
- [ ] **Session helpers** (2 endpoints)
  - `sessions/generate_title` - Generate a title automatically
  - `sessions/pin` / `sessions/unpin` - Pin operation

- [ ] **Configuration extensions** (1 week)
  - Extend `settings/get` to return more fields
  - Add `providers/list` to list available Providers

**Owner:** Backend Team  
**Dependencies:** None

#### 1.2 Domain-Model Definition (3-5 Days)
- [ ] Define the `domain.Plan` type
- [ ] Define the `domain.PlanRevision` type
- [ ] Define the `domain.PlanReport` type
- [ ] Extend `domain.Approval` to support `domain='plan'`

**Owner:** Backend Team  
**Dependencies:** None

#### 1.3 Storage-Layer Implementation (1 Week)
- [ ] Define the `storage.PlanStore` interface
- [ ] Implement the SQLite backend (`storage/sqlite/plans.go`)
- [ ] Write storage-layer unit tests

**Owner:** Backend Team  
**Dependencies:** Domain-model definition complete

---

### Phase 2: Internationalization Migration (In Parallel, 1 Week)

#### 2.1 Merge Translation Files
- [ ] Extract all keys from Agent Diva's `en.ts`
- [ ] Extract all keys from Agent Diva's `zh.ts`
- [ ] Merge them into VIVY's `ui/src/app/i18n.ts`
- [ ] Resolve key-name conflicts
- [ ] Add missing translations

**Owner:** Frontend Team  
**Dependencies:** None

#### 2.2 i18n Runtime Enhancements
- [ ] Support namespaces (to avoid key-name conflicts)
- [ ] Support dynamic language-pack loading
- [ ] Add translation-completeness checks

**Owner:** Frontend Team  
**Dependencies:** 2.1 complete

---

### Phase 3: Styling-System Preparation (In Parallel, 1 Week)

#### 3.1 CSS Token Extensions
- [ ] Analyze Agent Diva's Tailwind class usage
- [ ] Define the corresponding CSS variables in `tokens.css`
- [ ] Add semantic class names for new components

**Owner:** Frontend Team  
**Dependencies:** None

#### 3.2 Theme-System Integration
- [ ] Merge Agent Diva's `useTheme` logic into `preferences.ts`
- [ ] Ensure dark/light modes switch correctly
- [ ] Add a system-theme listener

**Owner:** Frontend Team  
**Dependencies:** 3.1 complete

---

### Phase 4: Test-Framework Setup (In Parallel, 3-5 Days)

#### 4.1 Unit-Test Framework
- [ ] Install Vitest
- [ ] Configure TypeScript support
- [ ] Migrate Agent Diva's key unit tests

**Owner:** QA Team  
**Dependencies:** None

#### 4.2 E2E Test Framework
- [ ] Extend the existing Playwright configuration
- [ ] Write E2E tests for the core paths
  - Create session → send message → view response
  - Plan-approval workflow
  - Settings-modification workflow

**Owner:** QA Team  
**Dependencies:** Some Phase 1 endpoints complete

---

## Risks and Mitigation

### High Risk
1. **Designing the Plan domain model from scratch**
   - Risk: Poor design could cause rework later
   - Mitigation: Refer to Agent Diva's implementation; build the MVP first, then iterate

2. **Secrets-management conflict**
   - Risk: Agent Diva allows API Key persistence, while VIVY prohibits it
   - Mitigation: Design a temporary in-memory key-input flow and clearly inform users of the limitation

### Medium Risk
3. **Missing Channel management**
   - Risk: If VIVY truly lacks Channel support, it must be implemented from scratch
   - Mitigation: Confirm the current state first; if missing, mark it as a Phase 2 task

4. **Styling migration takes more effort than expected**
   - Risk: Tailwind → CSS Modules conversion takes time
   - Mitigation: Use a hybrid strategy and retain commonly used Tailwind class names

### Low Risk
5. **Internationalization key-name conflicts**
   - Risk: The two projects use different translation-key naming conventions
   - Mitigation: Isolate them with namespaces (such as `chat.sendMessage` vs `settings.saveButton`)

---

## Next Actions

1. **Start immediately:** Phase 1.1 RPC endpoint additions (Plan-management MVP)
2. **Run in parallel:** Phase 2 internationalization migration + Phase 3 styling preparation
3. **Complete this week:** Confirm the current state of Channel management
4. **Review next week:** Phase 1 completion status and whether to proceed to the Phase 2 core chat-system migration

---

**Document Version:** v0.1  
**Created:** 2026-01-XX  
**Last Updated:** 2026-01-XX  
**Maintainer:** UI Migration Team  
**Status:** Draft - pending review
