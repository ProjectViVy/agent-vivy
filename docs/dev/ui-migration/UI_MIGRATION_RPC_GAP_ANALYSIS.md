# UI Migration RPC Endpoint Gap Analysis

This document compares the Tauri commands required by Agent Diva GUI with VIVY's existing JSON-RPC endpoints and identifies endpoints that need to be added.

## 1. Existing Endpoint Inventory (VIVY)

Based on the `Handle` method in `internal/rpc/control.go`:

### Session Management
- ✅ `session/create` - Create a session
- ✅ `session/list` - List sessions
- ✅ `session/get` - Get session details (including messages)
- ✅ `session/rename` - Rename a session
- ✅ `session/delete` - Delete a session
- ✅ `session/messages` - List session messages

### Run Control
- ✅ `preflight/run` - Run preflight checks
- ✅ `turn/start` - Start a conversation turn
- ✅ `turn/interrupt` / `run/cancel` - Cancel a run
- ✅ `run/get` - Get run status
- ✅ `run/subscribe` - Subscribe to the run event stream
- ✅ `run/unsubscribe` - Unsubscribe
- ✅ `run/log` - Get the run log

### Approval and Questions
- ✅ `approval/list` - List pending approvals
- ✅ `approval/respond` - Respond to an approval (approve/deny)
- ✅ `question/list` - List unanswered questions
- ✅ `question/respond` - Answer a question

### Review Center
- ✅ `review/list` - List review items
- ✅ `review/get` - Get review details
- ✅ `review/respond` - Respond to a review

### Subprocess Management
- ✅ `child/start` - Start a subprocess
- ✅ `child/get` - Get subprocess status
- ✅ `child/list` - List subprocesses
- ✅ `child/wait` - Wait for a subprocess to complete
- ✅ `child/cancel` - Cancel a subprocess

### Background Tasks
- ✅ `background/recover` - Recover background tasks
- ✅ `background/list` - List background tasks
- ✅ `background/attach` - Attach to a background task

### Studio-Related
- ✅ `generations/list` - List generations
- ✅ `generations/get` - Get generation details
- ✅ `generations/create` - Create a generation
- ✅ `generations/reject` - Reject a generation
- ✅ `evals/list` - List evaluations
- ✅ `evals/record` - Record an evaluation
- ✅ `evals/start` - Start an evaluation
- ✅ `promotions/list` - List promotions
- ✅ `promotions/promote` - Execute a promotion
- ✅ `species/inspect` - Inspect species status

### Settings
- ✅ `settings/get` - Get settings
- ✅ `settings/update` - Update settings

---

## 2. Endpoints Required by Agent Diva but Missing from VIVY

### 2.1 Plan Management (High Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `get_active_plan` | Get the session's current active plan | ❌ Missing | Add `plan/get_active` |
| `approve_active_plan_execution` | Approve the plan and continue execution | ❌ Missing | Add `plan/approve_execution` |
| `continue_approved_plan_execution` | Continue execution of an approved plan | ❌ Missing | Reuse `turn/start` + plan context |
| `get_plan_reports` | Get the plan-report list | ❌ Missing | Add `plan/list_reports` |
| `return_active_plan_to_draft` | Return the plan to draft | ❌ Missing | Add `plan/return_to_draft` |

**Implementation Recommendation:**
```go
// Add to internal/rpc/control.go:
case "plan/get_active":
    return h.getActivePlan(ctx, request)
case "plan/approve_execution":
    return h.approvePlanExecution(ctx, request)
case "plan/list_reports":
    return h.listPlanReports(ctx, request)
case "plan/return_to_draft":
    return h.returnPlanToDraft(ctx, request)
```

### 2.2 Configuration Management (Medium Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `get_runtime_config` | Get the complete runtime configuration | ⚠️ Partial (provider/model only) | Extend `settings/get` |
| `update_runtime_config` | Update the runtime configuration | ⚠️ Partial (provider/model only) | Extend `settings/update` |
| `list_providers` | List all Providers | ❌ Missing | Add `providers/list` |
| `add_provider` | Add a custom Provider | ❌ Missing | Add `providers/add` |
| `delete_provider` | Delete a Provider | ❌ Missing | Add `providers/delete` |
| `test_provider_connection` | Test the Provider connection | ❌ Missing | Add `providers/test` |

**Implementation Recommendation:**
Extend the existing `settings/get` and `settings/update` to include more configuration fields, or split them into dedicated `config/*` endpoints.

### 2.3 Channel Management (Medium Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `list_channels` | List channels | ✅ Exists (but is not exposed in the handler) | Confirm implementation |
| `create_channel` | Create a channel | ✅ Exists (but is not exposed in the handler) | Confirm implementation |
| `update_channel` | Update a channel | ❌ Missing | Add `channels/update` |
| `delete_channel` | Delete a channel | ❌ Missing | Add `channels/delete` |
| `test_channel_connection` | Test the channel connection | ❌ Missing | Add `channels/test` |

**Note:** No channel-related case was found in VIVY's `control.go`; confirm whether it is implemented elsewhere.

### 2.4 Skills Management (Low Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `list_skills` | List installed skills | ❌ Missing | Add `skills/list` |
| `install_skill` | Install a skill | ❌ Missing | Add `skills/install` |
| `uninstall_skill` | Uninstall a skill | ❌ Missing | Add `skills/uninstall` |
| `list_marketplace_skills` | List marketplace skills | ❌ Missing | Add `skills/marketplace` |

### 2.5 Memory and Persona (Low Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `list_memories` | List memory entries | ❌ Missing | Add `memories/list` |
| `get_memory` | Get memory details | ❌ Missing | Add `memories/get` |
| `delete_memory` | Delete a memory | ❌ Missing | Add `memories/delete` |
| `search_memories` | Search memories | ❌ Missing | Add `memories/search` |
| `get_persona` | Get the Persona | ❌ Missing | Add `persona/get` |
| `update_persona` | Update the Persona | ❌ Missing | Add `persona/update` |
| `get_evolution_proposals` | Get Evolution proposals | ❌ Missing | Add `evolution/list_proposals` |
| `apply_evolution_proposal` | Apply an Evolution proposal | ❌ Missing | Add `evolution/apply` |

### 2.6 Audit and Diagnostics (Low Priority)

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `get_token_stats` | Get Token statistics | ❌ Missing | Add `stats/tokens` |
| `get_audit_log` | Get the audit log | ❌ Missing | Add `audit/log` |
| `get_gui_log` | Get the GUI operation log | ❌ Missing | Add `audit/gui_log` |
| `get_raw_log` | Get the raw event stream | ⚠️ Partial (`run/log`) | Extend to cross-run queries |

### 2.7 Other Helper Endpoints

| Agent Diva Command | Purpose | VIVY Status | Recommended Approach |
|-------------------|------|----------|---------|
| `generate_session_title` | Generate a session title automatically | ❌ Missing | Add `sessions/generate_title` |
| `pin_session` | Pin a session | ❌ Missing | Add `sessions/pin` |
| `unpin_session` | Unpin a session | ❌ Missing | Add `sessions/unpin` |
| `get_compaction_status` | Get compaction status | ❌ Missing | Add `compaction/status` |
| `trigger_compaction` | Trigger compaction | ❌ Missing | Add `compaction/trigger` |

---

## 3. Recommended Implementation Priorities

### Phase 1A: Core-Functionality Requirements (Implement Immediately)
1. **Plan-management endpoints** (5)
   - `plan/get_active`
   - `plan/approve_execution`
   - `plan/list_reports`
   - `plan/return_to_draft`
   - `continue_approved_plan_execution` (can reuse `turn/start`)

2. **Session-title generation** (1)
   - `sessions/generate_title`

**Rationale:** These are core dependencies for the chat interface and plan-approval workflow; without them, basic conversation and plan-management functionality cannot be completed.

### Phase 1B: Configuration Management (Short-Term Implementation)
1. **Provider management** (4)
   - `providers/list`
   - `providers/add`
   - `providers/delete`
   - `providers/test`

2. **Extend settings endpoints**
   - Extend `settings/get` to return more configuration
   - Extend `settings/update` to support more fields

**Rationale:** Users need to be able to configure the LLM Provider and other runtime parameters.

### Phase 2: Channel and Skills (Medium-Term Implementation)
1. **Channel management** (5)
2. **Skills management** (4)

**Rationale:** These are core to a multi-channel gateway and extensibility, but are not immediately required for a single-user desktop scenario.

### Phase 3: Memory and Auditing (Long-Term Implementation)
1. **Memory management** (8)
2. **Audit logging** (4)

**Rationale:** These are advanced features that can be added incrementally after the core functionality is stable.

---

## 4. Data-Model Differences

### 4.1 Plan Data Structure

**Agent Diva's PlanRuntimeState:**
```typescript
interface PlanRuntimeState {
  plan_id: string;
  revision: number | null;
  title: string;
  goal: string;
  phase: 'Draft' | 'AwaitingApproval' | 'Execute' | 'Verify' | 'Completed' | 'Failed' | 'Partial';
  status: string;
  strategy: string | null;
  summary: string | null;
  markdown: string | null;
  validation_issues?: ValidationIssue[];
  steps: PlanStep[];
  todos: TodoItem[];
  created_at: string;
  updated_at: string;
  execution_id?: string | null;
}
```

**VIVY needs to define the corresponding domain types and storage interfaces.**

### 4.2 Approval Data-Structure Differences

**Agent Diva's ApprovalView:**
```typescript
interface ApprovalView {
  request_id: string;
  version: number;
  status: 'pending' | 'approved' | 'denied' | 'cancelled';
  domain: 'tool' | 'plan';
  resource: {
    session_id: string;
    resource_id: string;
    type: string;
  };
  presentation?: {
    session_key: string;
  };
  expires_at: string;
  // ...
}
```

**VIVY's approvalResult:**
```go
type approvalResult struct {
    ID         string       `json:"id"`
    RunID      domain.RunID `json:"run_id"`
    ToolCallID string       `json:"tool_call_id"`
    Decision   string       `json:"decision"`
    ExpiresAt  int64        `json:"expires_at"`
}
```

**Difference:** VIVY lacks fields such as `version`, `domain`, `resource`, and `presentation`; it must be extended to support plan approval.

---

## 5. Next Actions

1. **Confirm whether VIVY has a Plan domain model**
   - Check whether `internal/domain/` contains Plan-related types
   - If not, define `domain.Plan`, `domain.PlanRevision`, `domain.PlanReport`, and others

2. **Confirm whether VIVY's Approval Store supports Plan approvals**
   - Check the `storage.ApprovalStore` interface
   - Confirm whether it must be extended to support approvals with `domain='plan'`

3. **Design the plan-management storage layer**
   - Define the `storage.PlanStore` interface
   - Implement the SQLite backend

4. **Implement the missing RPC endpoints one by one**
   - In order from highest to lowest priority
   - Add unit tests for each endpoint

5. **Update the UI-layer `rpc.ts`**
   - Add wrappers for the new RPC calls
   - Ensure type safety

---

## 6. Risk Assessment

| Risk | Impact | Mitigation |
|------|------|---------|
| Plan domain model completely missing | High | Design from scratch, referring to Agent Diva's implementation |
| Approval Store does not support Plan | Medium | Extend the storage interface while preserving backward compatibility |
| Configuration management is complex | Medium | Implement in phases: support Provider first, then extend to others |
| Channel/Skills depend on external services | Low | Implement a stub first and integrate real services later |

---

**Document Version:** v0.1  
**Last Updated:** 2026-01-XX  
**Maintainer:** UI Migration Team
