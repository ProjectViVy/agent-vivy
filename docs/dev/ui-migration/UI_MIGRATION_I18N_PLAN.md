# UI Migration - Internationalization Migration Plan

## Overview

This document plans how to migrate Agent Diva's complete internationalization content to VIVY's simplified i18n system.

### Current-State Comparison

| Item | Agent Diva | VIVY | Gap |
|------|-----------|------|------|
| File size | en.ts: 1532 lines, zh.ts: ~1500 lines | i18n.ts: 326 lines (bilingual) | VIVY lacks many translation keys |
| Organization | Grouped by functional module (app/chat/settings, etc.) | Flat key namespace | Refactoring required |
| Language support | Chinese + English | Chinese + English | ✅ Consistent |
| Runtime | vue-i18n | Custom translate() function | ⚠️ Functional differences |
| Namespace | Deeply nested objects | Single-level keys | Adaptation required |

---

## 1. Agent Diva Translation-Key Inventory

### Identified Namespaces (Extracted from en.ts)

```typescript
{
  app: { ... },                    // application-level messages (welcome, errors, states)
  appDialog: { ... },              // dialog buttons and titles
  emotion: { ... },                // emotional states
  chat: { ... },                   // chat interface (input, tool calls, reasoning)
  convSidebar: { ... },            // conversation sidebar
  settings: { ... },               // general settings
  language: { ... },               // language settings
  theme: { ... },                  // theme settings
  sandbox: { ... },                // sandbox policy
  compaction: { ... },             // context compaction
  approvalCenter: { ... },         // approval center
  askUserQuestion: { ... },        // user questions
  planExecution: { ... },          // plan execution
  notebook: { ... },               // notebook view
  personaMemory: { ... },          // persona memory
  evolution: { ... },              // evolution proposals
  skills: { ... },                 // Skills marketplace
  mcp: { ... },                    // MCP servers
  channels: { ... },               // channel management
  providers: { ... },              // Provider configuration
  console: { ... },                // console view
  cronTasks: { ... },              // Cron tasks
  gatewayControl: { ... },         // Gateway control panel
  audit: { ... },                  // audit log
  tokenStats: { ... },             // Token statistics
  mate: { ... },                   // Desktop Mate (excluded)
  vrm: { ... },                    // VRM Avatar (excluded)
  voice: { ... },                  // voice interaction (excluded)
}
```

**Estimate:** Approximately 400-500 unique translation keys (approximately 350-400 excluding PET-related keys)

---

## 2. VIVY Existing Translation-Key Inventory

### Existing Key Names (en section)

```typescript
[
  "appName", "newSession", "sessions", "activity", "noActiveRuns",
  "openRun", "attachRun", "childRuns", "noChildRuns", "refreshChildren",
  "childLoading", "childLoadError", "childCancel", "childCancelling",
  "childResult", "childError", "depth", "createFirstSession", "noSessions",
  "sessionUntitled", "sessionCreated", "rename", "delete", "more",
  "renameSession", "renameSessionDescription", "deleteSession",
  "deleteSessionDescription", "sessionTitle", "save", "cancel",
  "confirmDelete", "retry", "loading", "refreshing", "loadingSession",
  "sessionLoadError", "sessionEmptyHint", "selectSessionHint",
  "noMessages", "noMessagesHint", "sendPlaceholder", "send", "planMode",
  "planModeHint", "run", "runStatus", "runAccepted", "runQueued",
  "runActive", "runCompleted", "runFailed", "runCancelled", "cancelRun",
  "cancelling", "inspector", "showInspector", "hideInspector", "overview",
  "events", "noRun", "noEvents", "eventCount", "startedAt", "provider",
  "model", "mode", "connection", "connected", "reconnecting",
  "waitingApproval", "waitingQuestion", "approvalRequired",
  "approvalDescription", "tool", "arguments", "expiresAt", "approve",
  "deny", "approving", "questionRequired", "answerPlaceholder",
  "answerLabel", "reviewReasonLabel", "reviewReasonPlaceholder",
  "answer", "answering", "preflightWarnings", "preflightDescription",
  "continue", "blocked", "runFailedCause", "providerError", "toolError",
  "internalError", "cancelledError", "unknownError", "eventDetails",
  "rawPayload", "locale", "languageChinese", "languageEnglish", "theme",
  "themeSystem", "themeLight", "themeDark", "menuSettings", "actionFailed",
  "unknownTool", "reviewCenter", "reviewCenterTitle", "refreshReviews",
  "backToChat", "noReviews", "noReviewsHint", "studio", "studioTitle",
  "studioIdentity", "studioGenerations", "studioNoGenerations",
  "studioNoGenerationsHint", "studioPromote", "studioReject",
  "studioPromoting", "studioRejecting", "studioAppliesNextLaunch",
  "studioBinary", "studioGeneration", "studioPolicy", "studioTools",
  "studioPhase", "studioEval", "studioRecipe", "refreshStudio",
  "studioLoadError", "settings", "settingsTitle", "settingsModelProvider",
  "settingsDefaultModel", "settingsBaseURL", "settingsBaseURLHint",
  "settingsProviderOpenAI", "settingsProviderAnthropic",
  "settingsConfigDefault", "settingsAppearance",
  "settingsSave", "settingsSaved", "settingsSaving", "settingsCancel",
  "settingsReadOnly", "settingsFallbackHint", "settingsRestartNote"
]
```

**Count:** Approximately 100 unique keys

---

## 3. Migration Strategy

### 3.1 Namespace Design

**Problem:** VIVY uses flat key names (such as `newSession`), while Agent Diva uses nested namespaces (such as `convSidebar.newSession`).

**Option A: Keep VIVY's Flat Style**
```typescript
// Advantages: simple and direct
// Disadvantages: key names may collide and be hard to maintain
messages = {
  "zh-CN": {
    newSession: "New session",
    chatNewSession: "New conversation",  // distinguish by prefix
    settingsNewSession: "Create new session",
  }
}
```

**Option B: Introduce a Namespace Separator**
```typescript
// Advantages: clear structure and easy maintenance
// Disadvantages: requires changing the translate() function
messages = {
  "zh-CN": {
    "convSidebar.newSession": "New conversation",
    "chat.placeholder": "Enter a message...",
  }
}

// translate() must support dot notation
translate(locale, "convSidebar.newSession")
```

**Option C: Hybrid Namespace (Recommended)**
```typescript
// Keep VIVY's existing top-level key style
// Use namespace prefixes for new features
messages = {
  "zh-CN": {
    // Keep existing keys unchanged
    newSession: "New session",
    sessions: "Sessions",
    
    // Use prefixes for added migration keys
    chatPlaceholder: "Enter a message...",
    chatSend: "Send",
    settingsTitle: "Settings",
  }
}
```

**Decision: Option C** - Minimize breaking changes while retaining extensibility

---

### 3.2 Key-Name Mapping

#### 3.2.1 Direct Reuse (No Migration Required)

The following Agent Diva keys already have equivalents in VIVY:

| Agent Diva | VIVY | Notes |
|-----------|------|------|
| `convSidebar.newSession` | `newSession` | ✅ |
| `convSidebar.sessions` | `sessions` | ✅ |
| `convSidebar.rename` | `rename` | ✅ |
| `convSidebar.delete` | `delete` | ✅ |
| `settings.save` | `save` | ✅ |
| `settings.cancel` | `cancel` | ✅ |
| `app.errorPrefix` | ❌ Missing | Must be added |
| `chat.send` | `send` | ✅ |
| `chat.planMode` | `planMode` | ✅ |

#### 3.2.2 Keys to Add

**Chat Interface (`chat` namespace)**
```typescript
// Approximately 80 new keys
chatPlaceholder: "Enter a message... (Enter to send)",
start: "Start chatting with DIVA~",
toolRunning: "Calling tool...",
toolCall: "Calling tool: {name}",
cleanToolCall: "Tool call: {name}",
toolSuccess: "Success",
toolFailed: "Failed",
viewDetails: "View details",
hideDetails: "Hide details",
inputArgs: "Input arguments:",
execResult: "Execution result:",
artifactSize: "Result size:",
artifactId: "Artifact ID:",
readHint: "Read hint:",
copyArtifactReference: "Copy artifact reference",
compactionRunning: "Compacting context…",
compactionCompleted: "Context compaction complete.",
compactionFailed: "Context compaction failed; original context retained.",
thinking: "Thinking deeply...",
retrying: "No response; retrying ({attempt}/{max})",
stalled: "Still waiting for the provider response...",
thinkingProcess: "Reasoning process",
thoughtProcess: "Chain of thought",
clearChat: "Clear chat",
historySessions: "Session history",
noHistory: "No history yet",
deleteSession: "Delete",
confirmDeleteSession: "Delete this session? This action cannot be undone.",
unknownTime: "Unknown time",
stopGeneration: "Stop generation",
emptyModels: "No saved models\nAdd one in Settings",
manageModels: "Manage models...",
rawMeta: "Structured metadata",
viewRawMeta: "View raw metadata",
hideRawMeta: "Hide raw metadata",
prefAutoReasoning: "Expand reasoning automatically",
prefAutoTool: "Expand tool details automatically",
prefAutoRawMeta: "Show raw metadata automatically",
copy: "Copy",
copied: "Copied",
edit: "Edit",
regenerate: "Regenerate",
rewind: "Rewind to here",
fork: "Fork from here",
saveMemory: "Save to memory",
pending: "Pending",
autoSelect: "Auto-select",
attachment: "Attachment",
voice: "Voice",
agentMode: "Agent mode",
askMode: "Ask mode",
conciseMode: "Concise mode",
deepThinking: "Deep thinking",
thinkingLow: "Low",
thinkingMedium: "Medium",
thinkingHigh: "High",
contextUsage: "Context usage",
queue: "Queue",
stop: "Stop",
agentModeDesc: "Execute tasks directly",
planModeDesc: "Plan first, then execute",
askModeDesc: "Read-only analysis mode",
permissionCautious: "Cautious",
permissionCautiousDesc: "All operations require confirmation",
permissionSmart: "Smart",
permissionSmartDesc: "Automatically approve low-risk operations",
permissionTrusted: "Trusted",
permissionTrustedDesc: "Only high-risk operations require confirmation",
readonly: "Read-only",
enterToSend: "Enter to send; Shift+Enter for a new line",
reasoningSteps: "{count} reasoning steps",
reasoningProcessing: "Processing...",
reasoningThought: "Thought for {time} seconds",
reasoningProcessed: "Processed for {time} seconds",
reasoningNoThinking: "No reasoning",
thinkingMode: "Thinking mode",
thinkingModeAuto: "Auto",
thinkingModeOn: "On",
thinkingModeOff: "Off",
attachFile: "Attach file",
uploading: "Uploading...",
removeAttachment: "Remove attachment",
filePlaceholder: "[File]",
// ... more
```

**Session Sidebar (`convSidebar` namespace)**
```typescript
// Approximately 20 new keys
convSidebarTitle: "Session history",
convSidebarSearch: "Search sessions...",
convSidebarPinned: "Pinned",
convSidebarNoHistory: "No sessions yet",
convSidebarRefresh: "Refresh",
convSidebarNoResults: "No matches",
convSidebarPin: "Pin",
convSidebarUnpin: "Unpin",
convSidebarClose: "Close sidebar",
convSidebarOpen: "Open session history",
convSidebarStatusIdle: "Idle",
convSidebarStatusRunning: "Running",
convSidebarStatusCompleted: "Completed",
convSidebarStatusError: "Error",
convSidebarUntitled: "Untitled",
convSidebarJustNow: "Just now",
convSidebarMinutesAgo: "{count} minutes ago",
convSidebarHoursAgo: "{count} hours ago",
convSidebarDaysAgo: "{count} days ago",
convSidebarShowAll: "Show all",
convSidebarShowPinned: "Show pinned only",
```

**Settings Panel (`settings` namespace)**
```typescript
// Approximately 60 new keys
settingsGeneral: "General",
settingsMcp: "MCP",
settingsSkills: "Skills",
settingsProviders: "Providers",
settingsChannels: "Channels",
settingsNetwork: "Network",
settingsLanguage: "Language",
settingsMate: "Companion",  // ⚠️ PET-related, excluded
settingsAbout: "About",
settingsSelfEvolution: "Self-evolution",
settingsSaved: "Saved",
settingsAdd: "Add",
settingsEdit: "Edit",
settingsName: "Name",
settingsModel: "Model",
settingsApiKey: "API key",
settingsApiBase: "API address",
settingsProvider: "Provider",
settingsActions: "Actions",
settingsSandbox: "Sandbox",
settingsSandboxDesc: "Isolated execution environment and security policy",
// Provider management
providersList: "Configured providers",
providersAdd: "Add provider",
providersEdit: "Edit provider",
providersDelete: "Delete provider",
providersTest: "Test connection",
providersTestSuccess: "Connection successful",
providersTestFailed: "Connection failed",
providersCustom: "Custom provider",
providersBuiltIn: "Built-in provider",
// Channel management
channelsList: "Configured channels",
channelsAdd: "Add channel",
channelsEdit: "Edit channel",
channelsDelete: "Delete channel",
channelsTest: "Test connection",
channelsEnable: "Enable",
channelsDisable: "Disable",
// Skills marketplace
skillsInstalled: "Installed skills",
skillsMarketplace: "Skills marketplace",
skillsInstall: "Install",
skillsUninstall: "Uninstall",
skillsUpdate: "Update",
skillsSearch: "Search skills",
skillsNoInstalled: "No installed skills",
skillsNoMarketplace: "Marketplace failed to load",
// MCP servers
mcpServers: "MCP servers",
mcpAddServer: "Add server",
mcpEditServer: "Edit server",
mcpDeleteServer: "Delete server",
mcpConnected: "Connected",
mcpDisconnected: "Disconnected",
mcpConnecting: "Connecting",
mcpTools: "Available tools",
mcpResources: "Available resources",
// Audit log
auditLog: "Audit log",
auditGuiLog: "GUI operation log",
auditRawLog: "Raw event stream",
auditStructuredEvents: "Structured events",
auditSearch: "Search logs",
auditFilter: "Filter",
auditExport: "Export",
auditNoLogs: "No logs yet",
// Token statistics
tokenStats: "Token statistics",
tokenUsed: "Used",
tokenLimit: "Limit",
tokenRemaining: "Remaining",
tokenResetTime: "Reset time",
tokenWarning: "Warning threshold",
tokenCritical: "Critical threshold",
```

**Approval Center (`approvalCenter` namespace)**
```typescript
// Approximately 30 new keys
approvalCenterTitle: "Pending approvals",
approvalPending: "Pending",
approvalApproved: "Approved",
approvalDenied: "Denied",
approvalCancelled: "Cancelled",
approvalExpired: "Expired",
approvalApprove: "Approve",
approvalDeny: "Deny",
approvalCancel: "Cancel",
approvalGrantOnce: "Grant once",
approvalGrantAlways: "Always grant",
approvalToolName: "Tool name",
approvalArguments: "Arguments",
approvalRiskContext: "Risk context",
approvalDiff: "Diff",
approvalViewDiff: "View diff",
approvalEditAtSource: "Edit at source",
approvalStale: "Approval expired; refresh",
approvalOutcomeUnknown: "Approval outcome unknown",
approvalRequestFailed: "Request failed",
approvalPageLimit: "Page loading limit reached",
approvalNoPending: "No pending approvals",
approvalLoading: "Loading...",
approvalError: "Load failed: {error}",
approvalRefresh: "Refresh",
approvalFilterAll: "All",
approvalFilterPending: "Pending",
approvalFilterDecided: "Decided",
approvalSortByExpiry: "Sort by expiry",
approvalSortByCreated: "Sort by creation time",
```

**Plan Execution (`planExecution` namespace)**
```typescript
// Approximately 40 new keys (⚠️ depends on the backend Plan domain model)
planTitle: "Plan",
planGoal: "Goal",
planPhase: "Phase",
planStatus: "Status",
planRevision: "Revision",
planMarkdown: "Plan document",
planSummary: "Summary",
planStrategy: "Strategy",
planSteps: "Steps",
planTodos: "To-dos",
planCreatedAt: "Created",
planUpdatedAt: "Updated",
planAwaitingApproval: "Awaiting approval",
planExecuting: "Executing",
planVerifying: "Verifying",
planCompleted: "Completed",
planFailed: "Failed",
planPartial: "Partially completed",
planApprove: "Approve and execute",
planReject: "Reject",
planReturnToDraft: "Return to draft",
planCancel: "Cancel execution",
planValidationError: "Validation error",
planValidationWarning: "Validation warning",
planTodoPending: "Pending",
planTodoInProgress: "In progress",
planTodoCompleted: "Completed",
planTodoCancelled: "Cancelled",
planTodoBlocks: "Blocks",
planTodoBlockedBy: "Blocked by",
planMaterializeTodos: "Create to-dos",
planContextRetain: "Retain context",
planContextCompact: "Compact context",
planContextClear: "Clear context",
planExecutionStarted: "Plan execution started",
planExecutionFailed: "Plan execution failed: {error}",
planExecutionCompleted: "Plan execution completed",
```

**Memory and Persona (`memory/persona` namespace)**
```typescript
// Approximately 50 new keys
memoryTitle: "Memory management",
memoryShortTerm: "Short-term memory",
memoryLongTerm: "Long-term memory",
memoryCore: "Core memory",
memorySearch: "Search memory",
memoryNoResults: "No memories found",
memoryDelete: "Delete memory",
memoryEdit: "Edit memory",
memoryCreatedAt: "Created",
memoryUpdatedAt: "Updated",
memoryConfidence: "Confidence",
memorySource: "Source",
personaTitle: "Persona workspace",
personaFrozenCore: "Frozen core",
personaEditable: "Editable portion",
personaSave: "Save persona",
personaRevert: "Revert",
personaVersionHistory: "Version history",
personaCurrentVersion: "Current version",
personaCreatedBy: "Created by",
personaLastModified: "Last modified",
evolutionTitle: "Evolution proposals",
evolutionProposals: "Proposals for review",
evolutionNoProposals: "No proposals for review",
evolutionApply: "Apply",
evolutionReject: "Reject",
evolutionPreview: "Preview changes",
evolutionDiff: "Diff",
evolutionRiskAnalysis: "Risk analysis",
evolutionApplied: "Proposal applied",
evolutionRejected: "Proposal rejected",
notebookTitle: "Notebook",
notebookSearch: "Search across sessions",
notebookNoResults: "No related content found",
notebookHits: "{count} matches",
notebookViewSource: "View source",
```

**Console and Diagnostics (`console` namespace)**
```typescript
// Approximately 30 new keys
consoleTitle: "Console",
consoleGatewayStatus: "Gateway status",
consoleHealthy: "Healthy",
consoleUnhealthy: "Unhealthy",
consoleChannels: "Channel connections",
consoleChannelConnected: "Connected",
consoleChannelDisconnected: "Disconnected",
consoleCronTasks: "Scheduled tasks",
consoleNoTasks: "No scheduled tasks",
consoleLogs: "Logs",
consoleLogLevel: "Log level",
consoleLogInfo: "Info",
consoleLogDebug: "Debug",
consoleLogError: "Error",
consoleLogSearch: "Search logs",
consoleLogStreaming: "Live stream",
consoleLogPaused: "Paused",
consoleTokenStats: "Token statistics",
consoleTokenUsage: "Usage",
consoleTokenBudget: "Budget",
consoleTokenWarning: "Warning",
consoleConfigEditor: "Configuration editor",
consoleConfigReadOnly: "Read-only mode",
consoleCannotEdit: "Cannot edit in production",
```

**Welcome Wizard (`onboarding` namespace)**
```typescript
// Approximately 20 new keys
onboardingWelcome: "Welcome to Vivy",
onboardingGetStarted: "Get started",
onboardingSkip: "Skip",
onboardingNext: "Next",
onboardingBack: "Back",
onboardingFinish: "Finish",
onboardingStep1Title: "Configure an LLM provider",
onboardingStep1Desc: "Choose an AI model provider and enter an API key",
onboardingStep2Title: "Configure search services",
onboardingStep2Desc: "Bocha Search enhances web search capabilities",
onboardingStep3Title: "Choose a navigation target",
onboardingStep3Desc: "Where would you like to start?",
onboardingNavigateChat: "Start chatting",
onboardingNavigateProviders: "Manage providers",
onboardingNavigateNetwork: "Network settings",
onboardingNavigateConsole: "Console",
onboardingApiKeyHint: "API keys are stored only in memory and must be re-entered after restart",
onboardingTestConnection: "Test connection",
onboardingConnectionSuccess: "Connection successful",
onboardingConnectionFailed: "Connection failed: {error}",
```

---

## 4. Implementation Steps

### Step 1: Extend the i18n.ts Structure (1 Day)

**Tasks:**
1. Keep the existing `messages` object structure
2. Add new translation keys to both the `zh-CN` and `en` language packs
3. Ensure type safety (update the `TranslationKey` type)

**Example Code:**
```typescript
const messages = {
  "zh-CN": {
    // === existing keys remain unchanged ===
    appName: "Vivy",
    newSession: "New session",
    // ... other existing keys
    
    // === added: chat interface ===
    chatPlaceholder: "Enter a message... (Enter to send)",
    chatStart: "Start chatting with Vivy~",
    chatToolRunning: "Calling tool...",
    // ... more chat keys
    
    // === added: conversation sidebar ===
    convSidebarTitle: "Session history",
    convSidebarSearch: "Search sessions...",
    // ... more convSidebar keys
    
    // === added: settings panel ===
    settingsGeneral: "General",
    settingsMcp: "MCP",
    // ... more settings keys
    
    // === added: approval center ===
    approvalCenterTitle: "Pending approvals",
    approvalPending: "Pending",
    // ... more approval keys
    
    // === added: plan execution ===
    planTitle: "Plan",
    planGoal: "Goal",
    // ... more plan keys
    
    // === added: memory and Persona ===
    memoryTitle: "Memory management",
    personaTitle: "Persona workspace",
    // ... more memory/persona keys
    
    // === added: console ===
    consoleTitle: "Console",
    consoleGatewayStatus: "Gateway status",
    // ... more console keys
    
    // === added: welcome wizard ===
    onboardingWelcome: "Welcome to Vivy",
    onboardingGetStarted: "Get started",
    // ... more onboarding keys
  },
  en: {
    // corresponding English translations
    // ...
  },
} as const;
```

### Step 2: Update the translate() Function to Support Interpolation (Half Day)

**Current Implementation:**
```typescript
export function translate(locale: Locale, key: TranslationKey, values?: Record<string, string | number>): string {
  let text: string = messages[locale][key] ?? messages.en[key];
  for (const [name, value] of Object.entries(values ?? {})) text = text.replace(`{${name}}`, String(value));
  return text;
}
```

**Problem:** If a key is not present in the specified language, the function falls back to English. If it is also missing in English, it returns `undefined`, causing a runtime error.

**Improvement:**
```typescript
export function translate(locale: Locale, key: TranslationKey, values?: Record<string, string | number>): string {
  const localeMessages = messages[locale];
  const fallbackMessages = messages.en;
  
  let text: string | undefined = localeMessages[key];
  if (text === undefined) {
    text = fallbackMessages[key];
    if (text === undefined) {
      console.warn(`Missing translation for key: ${key}`);
      return key;  // return the key name as a placeholder
    }
  }
  
  if (values) {
    for (const [name, value] of Object.entries(values)) {
      text = text.replace(`{${name}}`, String(value));
    }
  }
  
  return text;
}
```

### Step 3: Write the Translation-Completeness Check Script (Half Day)

**Purpose:** Ensure that the keys in `zh-CN` and `en` are identical to prevent omissions.

**Script Example:**
```typescript
// scripts/check-i18n-completeness.ts
import { messages } from "../src/app/i18n";

const zhKeys = new Set(Object.keys(messages["zh-CN"]));
const enKeys = new Set(Object.keys(messages.en));

const missingInZh = [...enKeys].filter(key => !zhKeys.has(key));
const missingInEn = [...zhKeys].filter(key => !enKeys.has(key));

if (missingInZh.length > 0) {
  console.error("❌ Missing in zh-CN:", missingInZh);
}
if (missingInEn.length > 0) {
  console.error("❌ Missing in en:", missingInEn);
}
if (missingInZh.length === 0 && missingInEn.length === 0) {
  console.log("✅ All translations are complete");
}
```

### Step 4: Manually Review Translation Quality (1-2 Days)

**Checks:**
1. **Tone consistency:** Agent Diva uses an affectionate tone ("Master~ 💕"); VIVY should remain professional and concise
2. **Terminology consistency:** Ensure the same concepts use the same translations in different places
3. **Length suitability:** English translations should not be too long, to avoid UI overflow
4. **Cultural adaptation:** Some expressions may require localization adjustments

**Example Adjustment:**
```typescript
// Agent Diva (too affectionate)
welcome: 'Hello, Master~ 💕\n\nI am DIVA. I will always be with you...'

// VIVY (professional and concise)
welcome: 'Hello, I am Vivy. How can I help you?'
```

### Step 5: Add Translation-Key Documentation (Half Day)

**Purpose:** Provide subsequent developers with a guide to using translation keys.

**Document Structure:**
```markdown
# Translation Key Usage Guide

## Naming conventions
- Use camelCase: `chatPlaceholder` rather than `chat_placeholder`
- Group by feature prefix: `chat*`, `settings*`, `approval*`
- Avoid duplication: reuse existing keys when possible

## Interpolation syntax
Use the `{variableName}` placeholder:
```typescript
translate(locale, "chatRetrying", { attempt: "1", max: "3" })
// "No response; retrying (1/3)"
```

## Adding a new key
1. Add it to both `zh-CN` and `en`
2. Run the completeness-check script
3. Update this document's key list
```

---

## 5. Effort Estimate

| Task | Effort | Owner |
|------|------|--------|
| Step 1: Extend the i18n.ts structure | 1 day | Frontend |
| Step 2: Update the translate() function | 0.5 day | Frontend |
| Step 3: Write the completeness-check script | 0.5 day | Frontend |
| Step 4: Manually review translation quality | 1-2 days | Frontend + PM |
| Step 5: Add translation-key documentation | 0.5 day | Frontend |
| **Total** | **3.5-4.5 days** | |

---

## 6. Acceptance Criteria

- [ ] `zh-CN` and `en` have exactly the same number of keys
- [ ] All translation keys have non-empty string values
- [ ] The completeness-check script passes
- [ ] A random sample of 20 keys is manually reviewed and meets the quality bar
- [ ] Translation-key documentation is updated
- [ ] Places in the UI that use the new translation keys display correctly (no `undefined`)

---

## 7. Follow-up Optimization Recommendations

1. **Automated translation synchronization:** Use a tool such as i18next-parser to extract translation keys from code automatically
2. **Translation-management platform:** Consider using a professional platform such as Lokalise/Crowdin for collaborative translation
3. **Multilingual support:** Japanese, Korean, and other Asian languages can be added in the future
4. **Dynamic loading:** For large applications, language packs can be loaded on demand to reduce initial size

---

**Document Version:** v0.1  
**Created:** 2026-01-XX  
**Maintainer:** UI Migration Team  
**Status:** Draft - pending review
