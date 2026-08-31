# Vivy New UI Architecture (React + TypeScript)

**Version:** 1.0  
**Date:** 2026-08-17  
**Status:** Design Phase  
**Scope:** Merge old `ui/` (vanilla TS with real RPC/SSE) and `new-ui/` (React with mock API) into a single production React application

---

## Executive Summary

This document defines the architecture for unifying two existing codebases:

1. **Old `ui/`**: Vanilla DOM manipulation with real JSON-RPC over WebSocket, SSE event subscriptions, and direct backend calls via `rpc.ts`, `sse.ts`, and `api.ts`. Complete state management in `controller.ts` and `store.ts`.

2. **New `new-ui/`**: Modern React + TanStack Router + shadcn/ui component library, but backed by mock API (`mock-api.ts`) using localStorage for persistence.

### Goal

Create a **single React-based UI** that:
- Uses **real backend communication** from old `ui/` (WebSocket JSON-RPC, SSE, preflight, approvals, settings)
- Adopts **React component structure** from new-ui/ (TanStack Router, shadcn/ui components)
- Maintains **demo/local simulation** for Persona, Notebook, Cron features with centralized `vivy.demo.*` localStorage keys
- Removes legacy entries (pet/dashboard/memory/MCP) and non-functional buttons (attachment/drawing/voice)
- Establishes **Vivy** as unified identity (no Agent-Diva/DiVA branding)

---

## 1. Directory Structure

```
ui/
├── package.json                    # npm workspace root
├── package-lock.json               # npm lockfile (delete pnpm-lock.yaml)
├── tsconfig.json                   # TypeScript config (strict mode)
├── vite.config.ts                  # Vite bundler configuration
├── index.html                      # Entry HTML
│
├── src/
│   ├── main.tsx                    # React entry point, router initialization
│   ├── router.tsx                  # TanStack Router instance
│   ├── routeTree.gen.ts            # Auto-generated route tree (DO NOT EDIT)
│   │
│   ├── lib/
│   │   ├── types.ts                # Unified type definitions (merge old api.ts + new types.ts)
│   │   ├── rpc.ts                  # WebSocket JSON-RPC client (from old ui/)
│   │   ├── sse.ts                  # Run event subscription manager (from old ui/)
│   │   ├── api.ts                  # Typed RPC client methods (from old ui/, adapted to hooks)
│   │   ├── demo-store.ts           # Centralized demo data layer (Persona/Notebook/Cron)
│   │   └── utils.ts                # Utility functions (date formatting, etc.)
│   │
│   ├── hooks/
│   │   ├── useRpcClient.ts         # RpcClient singleton hook with connection state
│   │   ├── useSession.ts           # Session CRUD + selection (real API, no duplicate state)
│   │   ├── useChat.ts              # Chat logic: send message, subscribe run, stream events
│   │   ├── useApprovals.ts         # Approval/question polling and response
│   │   ├── useSettings.ts          # Settings get/update (real API)
│   │   ├── useReviews.ts           # Review center list/respond
│   │   ├── useStudio.ts            # Studio generation/eval/promotion
│   │   ├── useChildren.ts          # Child run management
│   │   ├── useBackgroundRuns.ts    # Background run attachment
│   │   ├── useDemoPersona.ts       # Persona memory demo (localStorage vivy.demo.*)
│   │   ├── useDemoNotebook.ts      # Notebook reports demo (localStorage vivy.demo.*)
│   │   └── useDemoCron.ts          # Cron tasks demo (localStorage vivy.demo.*)
│   │
│   ├── store/
│   │   └── globalStore.ts          # Single app-level Zustand/Jotai store (unified state)
│   │
│   ├── components/
│   │   ├── ui/                     # shadcn/ui components (from new-ui/)
│   │   │   ├── button.tsx
│   │   │   ├── dialog.tsx
│   │   │   ├── input.tsx
│   │   │   ├── textarea.tsx
│   │   │   ├── select.tsx
│   │   │   ├── tabs.tsx
│   │   │   ├── badge.tsx
│   │   │   ├── card.tsx
│   │   │   ├── scroll-area.tsx
│   │   │   ├── skeleton.tsx
│   │   │   ├── tooltip.tsx
│   │   │   ├── dropdown-menu.tsx
│   │   │   ├── sheet.tsx
│   │   │   ├── sidebar.tsx
│   │   │   └── ...                 # Other shadcn components
│   │   │
│   │   ├── layout/
│   │   │   ├── AppLayout.tsx       # Main shell: sidebar + header + content area
│   │   │   ├── Sidebar.tsx         # Collapsible left sidebar (sessions, nav)
│   │   │   ├── Header.tsx          # Top bar: status, theme toggle, locale
│   │   │   └── SessionDrawer.tsx   # Right-side session list drawer
│   │   │
│   │   ├── chat/
│   │   │   ├── ChatView.tsx        # Main chat interface
│   │   │   ├── MessageBubble.tsx   # Individual message rendering
│   │   │   ├── ChatInput.tsx       # Text input + send/cancel buttons
│   │   │   ├── ToolCallRenderer.tsx # Tool call status display
│   │   │   ├── ThinkingBlock.tsx   # Reasoning/thinking content display
│   │   │   └── EventLogPanel.tsx   # Live event log inspector
│   │   │
│   │   ├── approvals/
│   │   │   ├── ApprovalsView.tsx   # Unified review center
│   │   │   ├── ApprovalCard.tsx    # Individual approval item
│   │   │   └── QuestionDialog.tsx  # AskUserQuestion modal
│   │   │
│   │   ├── settings/
│   │   │   ├── SettingsView.tsx    # Settings page
│   │   │   ├── ProviderSelect.tsx  # Model provider configuration
│   │   │   └── ThemeLocaleToggle.tsx # Theme/language controls
│   │   │
│   │   ├── studio/
│   │   │   ├── StudioView.tsx      # Species inspect/generations/evals
│   │   │   ├── GenerationTable.tsx
│   │   │   └── PromotionPanel.tsx
│   │   │
│   │   ├── runs/
│   │   │   ├── RunInspector.tsx    # Side panel showing run details
│   │   │   ├── ChildrenList.tsx    # Child run tree
│   │   │   └── BackgroundRunList.tsx # Active background runs
│   │   │
│   │   └── demo/
│   │       ├── DemoBanner.tsx      # "Demo / Local Simulation" banner
│   │       ├── PersonaMemoryView.tsx # Persona memory editor (demo)
│   │       ├── NotebookView.tsx    # Notebook reports viewer (demo)
│   │       └── CronTaskManagementView.tsx # Cron task manager (demo)
│   │
│   ├── routes/
│   │   ├── __root.tsx              # Root layout wrapper
│   │   ├── _layout.tsx             # Shared layout (sidebar + header)
│   │   ├── _layout.index.tsx       # Home → Chat view
│   │   ├── _layout.approvals.tsx   # Review Center (under layout)
│   │   ├── _layout.settings.tsx    # Settings (under layout)
│   │   ├── _layout.studio.tsx      # Vivy Studio (under layout)
│   │   ├── _layout.persona.tsx     # Persona Memory (DEMO - under layout)
│   │   ├── _layout.notebook.tsx    # Notebook (DEMO - under layout)
│   │   └── _layout.cron.tsx        # Cron Tasks (DEMO - under layout)
│   │
│   └── styles/
│       ├── globals.css             # Tailwind v4 base styles
│       └── themes.css              # Light/dark/system theme variables
│
├── public/
│   └── favicon.ico
│
└── tests/                          # Test directory (optional for now)
    ├── unit/
    └── e2e/
```

### Key Decisions

1. **Single `ui/` directory**: Replace both old `ui/` and `new-ui/` with one unified React app
2. **`lib/` contains infrastructure**: `rpc.ts`, `sse.ts`, `api.ts` are framework-agnostic utilities
3. **`hooks/` wraps API calls**: Each hook encapsulates async operations + React state
4. **`store/` for global state**: One source of truth (Zustand or Jotai), avoiding duplicate `useSession` patterns
5. **`components/` organized by feature**: Clear separation between real (chat/approvals/settings/studio) and demo (persona/notebook/cron)
6. **Routes under `_layout`**: All pages share common shell; demo pages marked with `DemoBanner`

---

## 2. State Management Strategy

### Problem: Duplicate `useSession` in Layout & Index

In new-ui/, both `ConversationSidebar` (via `_layout.tsx`) and `ChatView` (via index route) independently call `useSession()`, leading to:
- Double API calls on mount
- Inconsistent session selection state
- Race conditions during session switching

### Solution: Single Global Store

Use **Zustand** (lightweight, no provider needed) or **Jotai** (atomic, fine-grained reactivity) for app-level state.

#### Recommended: Zustand with Selectors

```typescript
// src/store/globalStore.ts
import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

export interface AppState {
  // Session state
  sessions: Session[];
  currentSessionId: string | null;
  sessionsPhase: AsyncPhase;
  sessionsError: string | null;

  // Chat/run state
  messages: Message[];
  currentRun: Run | null;
  streamingText: string;
  streamingReasoning: string;
  events: EventEnvelope[];

  // Inspector state
  inspectorOpen: boolean;
  inspectorTab: 'overview' | 'events';

  // UI state
  sidebarOpen: boolean;
  locale: Locale;
  theme: ThemeMode;

  // Actions
  actions: {
    setSessions: (sessions: Session[]) => void;
    setCurrentSessionId: (id: string | null) => void;
    addMessage: (message: Message) => void;
    updateStreamingText: (delta: string) => void;
    setInspectorOpen: (open: boolean) => void;
    // ... more actions
  };
}

export const useAppStore = create<AppState>()(
  subscribeWithSelector((set, get) => ({
    // Initial state
    sessions: [],
    currentSessionId: null,
    sessionsPhase: 'loading',
    sessionsError: null,
    messages: [],
    currentRun: null,
    streamingText: '',
    streamingReasoning: '',
    events: [],
    inspectorOpen: false,
    inspectorTab: 'overview',
    sidebarOpen: true,
    locale: 'en-US',
    theme: 'system',

    // Actions
    actions: {
      setSessions: (sessions) => set({ sessions }),
      setCurrentSessionId: (id) => set({ currentSessionId: id }),
      addMessage: (message) =>
        set((state) => ({ messages: [...state.messages, message] })),
      updateStreamingText: (delta) =>
        set((state) => ({ streamingText: state.streamingText + delta })),
      setInspectorOpen: (open) => set({ inspectorOpen: open }),
    },
  }))
);

// Selector hooks for fine-grained subscriptions
export const useCurrentSessionId = () =>
  useAppStore((state) => state.currentSessionId);

export const useMessages = () => useAppStore((state) => state.messages);
```

### Usage Pattern

```tsx
// Components read from store via selectors (only re-render when their slice changes)
function MessageList() {
  const messages = useMessages();
  return <div>{messages.map(msg => <MessageBubble key={msg.id} {...msg} />)}</div>;
}

// Hooks update store
async function useSendMessage() {
  const { setCurrentSessionId, addMessage, updateStreamingText } = useAppStore.getState().actions;
  // ... call API, update store
}
```

### Benefits

- **No duplicate fetches**: One hook loads sessions, updates store; all components read from store
- **Consistent state**: Session selection is authoritative in one place
- **Performance**: Selectors prevent unnecessary re-renders
- **Debugging**: Zustand DevTools shows full state tree

---

## 3. Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Backend (Go/vivy.exe)                    │
│  WebSocket JSON-RPC Server  │  HTTP Bootstrap (/rpc/bootstrap)│
└──────────────┬────────────────────────────┬──────────────────┘
               │                            │
               │ WebSocket                  │ HTTP (bootstrap only)
               ▼                            ▼
┌─────────────────────────┐    ┌──────────────────────────┐
│   lib/rpc.ts            │    │   (bootstrap token)       │
│   - RpcClient class     │◄───│                           │
│   - call<T>(method)     │    └──────────────────────────┘
│   - notify(method)      │
│   - onNotification()    │
└──────────┬──────────────┘
           │
           │ Typed wrappers
           ▼
┌─────────────────────────┐
│   lib/api.ts            │
│   - listSessions()      │
│   - createSession()     │
│   - postMessage()       │
│   - preflight()         │
│   - cancelRun()         │
│   - decideApproval()    │
│   - answerQuestion()    │
│   - getSettings()       │
│   - updateSettings()    │
│   - listReviews()       │
│   - respondReview()     │
│   - listChildren()      │
│   - startChild()        │
│   - inspectSpecies()    │
│   - promoteGeneration() │
└──────────┬──────────────┘
           │
           │ React hooks wrap API calls
           ▼
┌─────────────────────────────────────────────────────────────┐
│                        Hooks Layer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ useSession   │  │  useChat     │  │  useApprovals    │  │
│  │ - loadSessions│  │ - sendMessage│  │ - pollQuestions  │  │
│  │ - selectSession│ │ - subscribeRun│ │ - answerQuestion │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │             │
│  ┌──────▼─────────────────▼────────────────────▼─────────┐  │
│  │              store/globalStore.ts                      │  │
│  │         (Zustand: single source of truth)              │  │
│  └──────┬───────────────────────────────────────────────┘  │
│         │                                                   │
│         │ Components subscribe via selectors                │
│         ▼                                                   │
└─────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────┐
│                     Component Tree                          │
│                                                             │
│  AppLayout                                                  │
│  ├─ Sidebar                                                 │
│  │  ├─ SessionList (reads sessions from store)              │
│  │  └─ NavLinks                                             │
│  ├─ Header                                                  │
│  │  ├─ StatusBadge                                          │
│  │  └─ ThemeLocaleToggle                                    │
│  └─ Content Area                                            │
│     └─ <Outlet /> (TanStack Router)                         │
│        ├─ ChatView                                          │
│        │  ├─ MessageList (reads messages from store)        │
│        │  ├─ ChatInput (calls useSendMessage hook)          │
│        │  └─ EventLogPanel                                  │
│        ├─ ApprovalsView                                     │
│        │  └─ ApprovalCard (calls useApprovals hook)         │
│        ├─ SettingsView                                      │
│        │  └─ ProviderSelect (calls useSettings hook)        │
│        ├─ StudioView                                        │
│        ├─ PersonaMemoryView (DEMO)                          │
│        ├─ NotebookView (DEMO)                               │
│        └─ CronTaskManagementView (DEMO)                     │
└─────────────────────────────────────────────────────────────┘
```

### Key Flow: Send Message → Stream Response

1. User types message in `ChatInput`
2. `ChatInput` calls `useSendMessage(text)` hook
3. Hook calls `api.preflight(sessionId, text, mode)`
4. If preflight passes, hook calls `api.postMessage(sessionId, text, mode)`
5. Hook receives `{ run_id, status }`, updates store:
   - Sets `currentRun`
   - Adds user message to `messages`
6. Hook calls `subscribeRun(runId, 0, onEvent)` from `lib/sse.ts`
7. `sse.ts` opens WebSocket notification listener for `run/event`
8. On each event:
   - `model.delta` → append to `streamingText` in store
   - `tool.requested` → add tool call placeholder to last assistant message
   - `tool.finished` → update tool call with result
   - `user.question_required` → set `pendingQuestion` in store
   - `run.completed/failed/cancelled` → finalize run, close subscription
9. Components subscribed to `streamingText` auto-update (React reactivity)

---

## 4. Component Hierarchy

```
App (main.tsx)
└─ RouterProvider (TanStack Router)
   └─ RouteContext
      └─ __root.tsx
         └─ _layout.tsx (shared shell)
            ├─ AppLayout
            │  ├─ Sidebar
            │  │  ├─ Logo/Brand ("Vivy")
            │  │  ├─ SessionSearch
            │  │  ├─ SessionList
            │  │  │  └─ SessionItem (pin/rename/delete)
            │  │  └─ NewSessionButton
            │  ├─ Header
            │  │  ├─ SidebarToggleButton
            │  │  ├─ StatusIndicator (online/offline)
            │  │  ├─ ModelInfo (deepseek-chat)
            │  │  └─ Actions
            │  │     ├─ SessionDrawerTrigger
            │  │     ├─ TodoDrawerTrigger
            │  │     └─ ThemeLocaleToggle
            │  └─ MainContent
            │     └─ Outlet (route-specific page)
            │
            ├─ ChatView (_layout.index.tsx)
            │  ├─ MessageList
            │  │  └─ MessageBubble (user/assistant/tool/system)
            │  │     ├─ TextContent (markdown rendered)
            │  │     ├─ ThinkingBlock (reasoning chain)
            │  │     └─ ToolCallRenderer
            │  │        └─ ToolStatusBadge (running/success/error)
            │  ├─ ChatInput
            │  │  ├─ TextArea
            │  │  ├─ ModeToggle (normal/plan)
            │  │  └─ SendButton / CancelButton
            │  └─ RunInspector (collapsible side panel)
            │     ├─ OverviewTab
            │     │  ├─ RunStatus
            │     │  ├─ ChildrenList
            │     │  └─ PendingApproval/Question Card
            │     └─ EventsTab
            │        └─ EventLogList
            │
            ├─ ApprovalsView (_layout.approvals.tsx)
            │  ├─ ReviewList
            │  │  └─ ReviewCard
            │  │     ├─ ApprovalActions (approve/deny)
            │  │     └─ QuestionAnswerForm
            │  └─ EmptyState
            │
            ├─ SettingsView (_layout.settings.tsx)
            │  ├─ ProviderConfigForm
            │  │  ├─ ProviderSelect
            │  │  ├─ ModelInput
            │  │  └─ BaseUrlInput
            │  ├─ ThemeSelector (system/light/dark)
            │  └─ LocaleSelector (zh-CN/en-US)
            │
            ├─ StudioView (_layout.studio.tsx)
            │  ├─ SpeciesInspectPanel
            │  ├─ GenerationsTable
            │  └─ PromotionPanel
            │
            ├─ PersonaMemoryView (_layout.persona.tsx) [DEMO]
            │  ├─ DemoBanner
            │  └─ PersonaEditor (7 kinds)
            │
            ├─ NotebookView (_layout.notebook.tsx) [DEMO]
            │  ├─ DemoBanner
            │  ├─ ReportList
            │  └─ SessionSearch
            │
            └─ CronTaskManagementView (_layout.cron.tsx) [DEMO]
               ├─ DemoBanner
               └─ CronJobList
```

### Component Responsibilities

| Component | Responsibility | Real/Demo |
|-----------|---------------|-----------|
| `ChatView` | Display messages, handle send, show streaming | Real |
| `ApprovalsView` | List pending reviews, approve/deny/answer | Real |
| `SettingsView` | Configure provider, model, base URL | Real |
| `StudioView` | Inspect species, manage generations | Real |
| `PersonaMemoryView` | Edit persona documents | Demo (localStorage) |
| `NotebookView` | View reports, search sessions | Demo (localStorage) |
| `CronTaskManagementView` | Manage cron jobs | Demo (localStorage) |

---

## 5. Routing Structure

Using **TanStack Router** with file-based routing convention:

```
src/routes/
├── __root.tsx              # Root layout (RouterProvider context)
├── _layout.tsx             # Shared shell (sidebar + header)
├── _layout.index.tsx       # → / (Chat view)
├── _layout.approvals.tsx   # → /approvals (Review Center)
├── _layout.settings.tsx    # → /settings (Settings)
├── _layout.studio.tsx      # → /studio (Vivy Studio)
├── _layout.persona.tsx     # → /persona (Persona Memory - DEMO)
├── _layout.notebook.tsx    # → /notebook (Notebook - DEMO)
└── _layout.cron.tsx        # → /cron (Cron Tasks - DEMO)
```

### Route Configuration

Each route file uses `createFileRoute`:

```tsx
// src/routes/_layout.index.tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_layout/')({
  component: ChatPage,
});

function ChatPage() {
  return <ChatView />;
}
```

### Navigation Items

| Path | Label | Icon | Demo? |
|------|-------|------|-------|
| `/` | Chat | MessageSquare | No |
| `/approvals` | Review Center | Bell | No |
| `/settings` | Settings | Settings | No |
| `/studio` | Vivy Studio | Cpu | No |
| `/persona` | Persona Memory | BrainCircuit | Yes |
| `/notebook` | Notebook | NotebookPen | Yes |
| `/cron` | Cron Tasks | Clock | Yes |

### Removed Routes (from old new-ui/)

- `/pet` → Deleted (no backend support)
- `/dashboard` → Deleted (no backend support)
- `/memory` → Merged into `/persona` (renamed)
- `/mcp` → Deleted (no backend support)
- `/skills` → Not yet implemented (future work)

---

## 6. File-by-File Migration Plan

### Phase 1: Infrastructure Setup

| Step | Action | Source | Target | Notes |
|------|--------|--------|--------|-------|
| 1.1 | Copy `rpc.ts` | `ui/src/rpc.ts` | `ui/src/lib/rpc.ts` | No changes needed |
| 1.2 | Copy `sse.ts` | `ui/src/sse.ts` | `ui/src/lib/sse.ts` | No changes needed |
| 1.3 | Copy `api.ts` | `ui/src/api.ts` | `ui/src/lib/api.ts` | Keep all exports, remove unused ones later |
| 1.4 | Merge types | `ui/src/api.ts` + `new-ui/src/lib/types.ts` | `ui/src/lib/types.ts` | Consolidate into single types file |
| 1.5 | Create `demo-store.ts` | New | `ui/src/lib/demo-store.ts` | Centralized localStorage with `vivy.demo.*` prefix |

### Phase 2: State Management

| Step | Action | Source | Target | Notes |
|------|--------|--------|--------|-------|
| 2.1 | Install Zustand | npm install zustand | - | Or Jotai if preferred |
| 2.2 | Create global store | `ui/src/app/store.ts` + new-ui patterns | `ui/src/store/globalStore.ts` | Merge old AppState with new-ui needs |
| 2.3 | Define selectors | New | `ui/src/store/globalStore.ts` | Export typed selector hooks |

### Phase 3: Hook Creation

| Step | Action | Source | Target | Notes |
|------|--------|--------|--------|-------|
| 3.1 | Create `useRpcClient` | `ui/src/rpc.ts` pattern | `ui/src/hooks/useRpcClient.ts` | Singleton RpcClient with connection state |
| 3.2 | Create `useSession` | new-ui `useSession.ts` + old controller | `ui/src/hooks/useSession.ts` | Use real `api.listSessions()`, update store |
| 3.3 | Create `useChat` | old controller `sendMessage()` | `ui/src/hooks/useChat.ts` | Integrate preflight, postMessage, subscribeRun |
| 3.4 | Create `useApprovals` | old controller approval logic | `ui/src/hooks/useApprovals.ts` | Poll `api.listQuestions()`, respond |
| 3.5 | Create `useSettings` | old controller settings logic | `ui/src/hooks/useSettings.ts` | Wrap `api.getSettings()` / `updateSettings()` |
| 3.6 | Create `useReviews` | old controller review logic | `ui/src/hooks/useReviews.ts` | List/respond reviews |
| 3.7 | Create `useStudio` | old controller studio logic | `ui/src/hooks/useStudio.ts` | Inspect species, generations, promotions |
| 3.8 | Create `useChildren` | old controller children logic | `ui/src/hooks/useChildren.ts` | List/cancel child runs |
| 3.9 | Create `useBackgroundRuns` | old controller background logic | `ui/src/hooks/useBackgroundRuns.ts` | Attach background runs |
| 3.10 | Create `useDemoPersona` | new-ui mock-api persona | `ui/src/hooks/useDemoPersona.ts` | Use `demo-store.ts` with `vivy.demo.persona.*` |
| 3.11 | Create `useDemoNotebook` | new-ui mock-api notebook | `ui/src/hooks/useDemoNotebook.ts` | Use `demo-store.ts` with `vivy.demo.notebook.*` |
| 3.12 | Create `useDemoCron` | new-ui mock-api cron | `ui/src/hooks/useDemoCron.ts` | Use `demo-store.ts` with `vivy.demo.cron.*` |

### Phase 4: Component Migration

| Step | Action | Source | Target | Notes |
|------|--------|--------|--------|-------|
| 4.1 | Copy shadcn/ui components | `new-ui/src/components/ui/` | `ui/src/components/ui/` | No changes needed |
| 4.2 | Adapt `ConversationSidebar` | `new-ui/src/components/chat/ConversationSidebar.tsx` | `ui/src/components/layout/Sidebar.tsx` | Remove pet/dashboard/memory/MCP links |
| 4.3 | Adapt `SessionDrawer` | `new-ui/src/components/chat/SessionDrawer.tsx` | `ui/src/components/layout/SessionDrawer.tsx` | Use store for sessions |
| 4.4 | Create `AppLayout` | new-ui `_layout.tsx` | `ui/src/components/layout/AppLayout.tsx` | Shared shell |
| 4.5 | Create `Header` | new-ui header section | `ui/src/components/layout/Header.tsx` | Status, theme, locale |
| 4.6 | Adapt `ChatView` | `new-ui/src/components/chat/ChatView.tsx` | `ui/src/components/chat/ChatView.tsx` | Connect to `useChat` hook, remove mock |
| 4.7 | Adapt `MessageBubble` | `new-ui/src/components/chat/MessageBubble.tsx` | `ui/src/components/chat/MessageBubble.tsx` | Render markdown, tool calls, thinking |
| 4.8 | Adapt `ChatInput` | `new-ui/src/components/chat/ChatInput.tsx` | `ui/src/components/chat/ChatInput.tsx` | Connect to `useChat.sendMessage()` |
| 4.9 | Create `ToolCallRenderer` | Old ui DOM renderer | `ui/src/components/chat/ToolCallRenderer.tsx` | Show tool status (running/success/error) |
| 4.10 | Create `ThinkingBlock` | Old ui reasoning display | `ui/src/components/chat/ThinkingBlock.tsx` | Collapsible reasoning chain |
| 4.11 | Create `EventLogPanel` | Old ui event log | `ui/src/components/chat/EventLogPanel.tsx` | Live event stream viewer |
| 4.12 | Adapt `ApprovalsView` | `new-ui/src/components/approvals/ApprovalsView.tsx` | `ui/src/components/approvals/ApprovalsView.tsx` | Connect to `useApprovals` |
| 4.13 | Create `ApprovalCard` | Old ui approval dialog | `ui/src/components/approvals/ApprovalCard.tsx` | Approve/deny UI |
| 4.14 | Create `QuestionDialog` | Old ui question prompt | `ui/src/components/approvals/QuestionDialog.tsx` | AskUserQuestion modal |
| 4.15 | Adapt `SettingsView` | `new-ui/src/components/settings/SettingsView.tsx` | `ui/src/components/settings/SettingsView.tsx` | Connect to `useSettings` |
| 4.16 | Create `ProviderSelect` | Old ui settings form | `ui/src/components/settings/ProviderSelect.tsx` | Provider/model/base URL inputs |
| 4.17 | Create `ThemeLocaleToggle` | Old ui preferences | `ui/src/components/settings/ThemeLocaleToggle.tsx` | Theme/locale selectors |
| 4.18 | Create `StudioView` | Old ui studio view | `ui/src/components/studio/StudioView.tsx` | Connect to `useStudio` |
| 4.19 | Create `RunInspector` | Old ui inspector | `ui/src/components/runs/RunInspector.tsx` | Side panel with overview/events tabs |
| 4.20 | Create `ChildrenList` | Old ui children display | `ui/src/components/runs/ChildrenList.tsx` | Child run tree |
| 4.21 | Create `BackgroundRunList` | Old ui background runs | `ui/src/components/runs/BackgroundRunList.tsx` | Active background runs |
| 4.22 | Create `DemoBanner` | New | `ui/src/components/demo/DemoBanner.tsx` | "Demo / Local Simulation" warning |
| 4.23 | Adapt `PersonaMemoryView` | `new-ui/src/components/persona-memory/PersonaMemoryView.tsx` | `ui/src/components/demo/PersonaMemoryView.tsx` | Connect to `useDemoPersona`, add DemoBanner |
| 4.24 | Adapt `NotebookView` | `new-ui/src/components/notebook/NotebookView.tsx` | `ui/src/components/demo/NotebookView.tsx` | Connect to `useDemoNotebook`, add DemoBanner |
| 4.25 | Adapt `CronTaskManagementView` | `new-ui/src/components/cron/CronTaskManagementView.tsx` | `ui/src/components/demo/CronTaskManagementView.tsx` | Connect to `useDemoCron`, add DemoBanner |

### Phase 5: Routing

| Step | Action | Source | Target | Notes |
|------|--------|--------|--------|-------|
| 5.1 | Create `__root.tsx` | new-ui | `ui/src/routes/__root.tsx` | Root wrapper |
| 5.2 | Create `_layout.tsx` | new-ui | `ui/src/routes/_layout.tsx` | Shared shell route |
| 5.3 | Create `_layout.index.tsx` | new-ui | `ui/src/routes/_layout.index.tsx` | Chat page |
| 5.4 | Create `_layout.approvals.tsx` | new-ui | `ui/src/routes/_layout.approvals.tsx` | Review center |
| 5.5 | Create `_layout.settings.tsx` | new-ui | `ui/src/routes/_layout.settings.tsx` | Settings page |
| 5.6 | Create `_layout.studio.tsx` | Old ui | `ui/src/routes/_layout.studio.tsx` | Studio page |
| 5.7 | Create `_layout.persona.tsx` | new-ui | `ui/src/routes/_layout.persona.tsx` | Persona (demo) |
| 5.8 | Create `_layout.notebook.tsx` | new-ui | `ui/src/routes/_layout.notebook.tsx` | Notebook (demo) |
| 5.9 | Create `_layout.cron.tsx` | new-ui | `ui/src/routes/_layout.cron.tsx` | Cron (demo) |
| 5.10 | Update `router.tsx` | new-ui | `ui/src/router.tsx` | Router instance with query client |

### Phase 6: Cleanup & Branding

| Step | Action | Details |
|------|--------|---------|
| 6.1 | Remove mock API | Delete `new-ui/src/lib/mock-api.ts` |
| 6.2 | Remove unused hooks | Delete `useSkills.ts` if not implementing skills page yet |
| 6.3 | Update branding | Change "Agent-Diva"/"DiVA"/"Meoo" → "Vivy" everywhere |
| 6.4 | Remove dead routes | Delete references to `/pet`, `/dashboard`, `/memory`, `/mcp` |
| 6.5 | Remove dead buttons | Delete attachment/drawing/voice buttons from `ChatInput` |
| 6.6 | Update package.json | Ensure npm as package manager, delete pnpm-lock.yaml |
| 6.7 | Generate package-lock.json | Run `npm install` to generate lockfile |

---

## 7. Key Interfaces and Types

### Unified Type Definitions (`src/lib/types.ts`)

Merge old `api.ts` types with new-ui `types.ts`:

```typescript
// From old ui/src/api.ts (keep these exactly)
export interface Session {
  id: string;
  title: string;
  created_at: number;
  pinned?: boolean;
  last_message?: string;
  message_count?: number;
  updated_at?: number;
}

export interface Message {
  id: string;
  run_id?: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  reasoning?: string;
  tool_calls?: ToolCall[];
  created_at: number;
}

export interface ToolCall {
  id: string;
  name: string;
  args: Record<string, unknown>;
  result?: string;
  status: 'running' | 'success' | 'error';
  error?: string;
}

export interface Run {
  id: string;
  session_id: string;
  status: RunStatus;
  created_at: number;
}

export type RunStatus = 'accepted' | 'queued' | 'active' | 'completed' | 'failed' | 'cancelled';
export type RunMode = 'normal' | 'plan';
// Entry assembly serving the run. Empty/omitted means 'web'.
export type Face = 'web' | 'tui' | 'code';

export interface EventEnvelope {
  run_id: string;
  seq: number;
  type: RunEventType;
  created_at: number;
  payload_version: number;
  payload: Record<string, unknown>;
}

export type RunEventType = 
  | 'run.started'
  | 'provider.retry'
  | 'provider.stall'
  | 'model.reasoning_delta'
  | 'model.delta'
  | 'model.usage'
  | 'model.completed'
  | 'model.request'
  | 'tool.requested'
  | 'tool.approval_required'
  | 'tool.approval_decided'
  | 'tool.approval_expired'
  | 'tool.approval_cancelled'
  | 'tool.proposal_stale'
  | 'tool.started'
  | 'tool.finished'
  | 'policy.evaluated'
  | 'hook.started'
  | 'hook.completed'
  | 'hook.blocked'
  | 'user.question_required'
  | 'user.question_answered'
  | 'user.question_cancelled'
  | 'user.question_expired'
  | 'child.requested'
  | 'child.started'
  | 'child.suspended'
  | 'child.resumed'
  | 'child.completed'
  | 'child.failed'
  | 'child.cancelled'
  | 'run.completed'
  | 'run.failed'
  | 'run.cancelled';

export interface Approval {
  id: string;
  run_id: string;
  tool_call_id: string;
  expires_at: number;
}

export interface Question {
  id: string;
  run_id: string;
  tool_call_id: string;
  prompt: string;
  status?: 'pending' | 'answered' | 'expired';
  expires_at: number;
}

export type ReviewKind = 'approval' | 'question';
export type ReviewStatus = 'pending' | 'approved' | 'denied' | 'answered' | 'cancelled' | 'expired' | 'stale';

export interface ReviewItem {
  id: string;
  kind: ReviewKind;
  status: ReviewStatus;
  session_id: string;
  session_title?: string;
  run_id: string;
  tool_call_id?: string;
  tool_name?: string;
  source?: string;
  actor?: string;
  created_at: number;
  expires_at: number;
  decided_at?: number;
  action?: string;
  target?: string;
  precondition_hash?: string;
  preview?: string;
  risk_findings?: string[];
  arguments?: Record<string, unknown>;
  prompt?: string;
  decision_reason?: string;
  stale_reason?: string;
  error?: string;
  effect?: string;
  reversibility?: string;
  scope?: string;
  trust?: string;
}

export interface ChildRun {
  id: string;
  parent_run_id: string;
  root_run_id: string;
  session_id: string;
  status: RunStatus;
  depth: number;
  workspace_id?: string;
  result?: string;
  error?: string;
  created_at: number;
}

export interface BackgroundRun extends Run {
  workspace_id?: string;
  events_url: string;
  logs_url: string;
}

export interface Settings {
  provider: string;
  default_model: string;
  base_url: string;
  read_only: boolean;
  config_provider: string;
  config_model: string;
}

export interface SpeciesInspect {
  protocol_version: string;
  binary_id: string;
  generation_id: string;
  artifact_sha256?: string;
  recipe: { loop?: string; world?: string; providers?: string[]; tools?: string[]; plugins?: string[] };
  policy_profile: string;
  policy_hash: string;
  tools: Array<{ name: string; readonly: boolean }>;
  grants: string[];
}

export interface Generation {
  id: string;
  parent_id?: string;
  artifact_sha256: string;
  source_ref?: string;
  recipe: SpeciesInspect['recipe'];
  phase: 'built' | 'evaluated' | 'promoted' | 'rejected';
  created_at: number;
}

export interface EvalRun {
  id: string;
  candidate_id: string;
  baseline_id?: string;
  suite: string;
  verdict: 'better' | 'worse' | 'mixed' | 'failed_to_run';
  journal_ref?: string;
  created_at: number;
}

export interface Promotion {
  id: string;
  from_id: string;
  to_id: string;
  eval_id: string;
  actor: string;
  phase: string;
  applies_at: string;
  created_at: number;
}

// From new-ui types (adapt for Vivy)
export interface Preflight {
  status: 'ready' | 'warning' | 'blocked';
  mode: RunMode;
  face?: Face;
  policy_profile: string;
  policy_hash?: string;
  selected_tools: string[];
  tool_decisions: Array<{ tool_name: string; decision: string; reason: string }>;
  context_bytes: number;
  hook_ready: boolean;
  warnings?: string[];
  blockers?: string[];
  next_actions?: string[];
}

// Demo types (for Persona/Notebook/Cron)
export type PersonaKind = 'identity' | 'relationship' | 'redline' | 'user' | 'world' | 'dream' | 'dark';

export interface PersonaDocument {
  kind: PersonaKind;
  content: string;
  revision: number;
  updated_at: string;
}

export type ReportPeriod = 'daily' | 'weekly' | 'monthly';

export interface NotebookReport {
  id: string;
  period: ReportPeriod;
  date: string;
  title: string;
  summary: string;
  content: string;
  generatedAt?: string;
  generatedBy?: string;
}

export type ScheduleKind = 'at' | 'every' | 'cron';
export type CronStatus = 'running' | 'scheduled' | 'paused' | 'completed' | 'failed';

export interface CronSchedule {
  kind: ScheduleKind;
  atMs?: number;
  everyMs?: number;
  expr?: string;
  tz?: string;
}

export interface CronPayload {
  kind: string;
  message: string;
  deliver: boolean;
  channel?: string;
  to?: string;
}

export interface CronJobDto {
  id: string;
  name: string;
  enabled: boolean;
  schedule: CronSchedule;
  payload: CronPayload;
  state: {
    nextRunAtMs?: number;
    lastRunAtMs?: number;
    lastStatus?: string;
    lastError?: string;
  };
  createdAtMs: number;
  updatedAtMs: number;
  deleteAfterRun: boolean;
  isRunning: boolean;
  activeRun?: CronRunSnapshot | null;
  computedStatus: CronStatus;
}

export interface CronRunSnapshot {
  run_id: string;
  job_id: string;
  startedAtMs: number;
  lastHeartbeatAtMs: number;
  trigger: 'scheduled' | 'manual';
  cancelable: boolean;
}
```

### API Function Signatures (`src/lib/api.ts`)

Keep all exports from old `api.ts`:

```typescript
// Sessions
export function listSessions(): Promise<{ sessions: Session[] }>;
export function createSession(title: string): Promise<Session>;
export function renameSession(id: string, title: string): Promise<Session>;
export function deleteSession(id: string): Promise<void>;
export function listMessages(sessionID: string): Promise<{ messages: Message[] }>;

// Runs
export function postMessage(sessionID: string, text: string, mode: RunMode, face?: Face): Promise<{ run_id: string; status: RunStatus }>;
export function preflight(sessionID: string, text: string, mode: RunMode, face?: Face): Promise<Preflight>;
export function getRun(runID: string): Promise<Run>;
export function getRunLog(runID: string, afterSeq?: number): Promise<{ events: RunLogEvent[] }>;
export function cancelRun(runID: string): Promise<{ run_id: string; status: string }>;

// Background runs
export function listBackgroundRuns(): Promise<{ runs: BackgroundRun[] }>;
export function attachBackgroundRun(runID: string): Promise<BackgroundRun>;

// Child runs
export function startChild(request: { parent_run_id: string; text: string; policy_profile?: string; tool_names?: string[] }): Promise<ChildRun>;
export function getChild(runID: string): Promise<ChildRun>;
export function listChildren(parentRunID: string, tree?: boolean): Promise<{ children: ChildRun[] }>;
export function waitChild(runID: string): Promise<ChildRun>;
export function cancelChild(runID: string): Promise<ChildRun>;

// Approvals & Questions
export function listApprovals(): Promise<{ approvals: Approval[] }>;
export function decideApproval(approvalID: string, decision: 'approved' | 'denied'): Promise<{ approval_id: string; run_id: string; decision: string }>;
export function listQuestions(): Promise<{ questions: Question[] }>;
export function answerQuestion(questionID: string, answer: string): Promise<{ question_id: string; run_id: string; answer: string }>;

// Reviews (unified)
export function listReviews(params?: { kind?: ReviewKind; status?: ReviewStatus; session_id?: string; limit?: number }): Promise<{ reviews: ReviewItem[] }>;
export function getReview(reviewID: string): Promise<ReviewItem>;
export function respondReview(reviewID: string, response: { action: 'approve' | 'deny' | 'answer' | 'cancel'; reason?: string; answer?: string }): Promise<{ review_id: string; status: string }>;

// Settings
export function getSettings(): Promise<Settings>;
export function updateSettings(params: { provider: string; default_model: string; base_url: string }): Promise<Settings>;

// Studio
export function inspectSpecies(): Promise<SpeciesInspect>;
export function listGenerations(): Promise<{ generations: Generation[] }>;
export function createGeneration(params: { artifact_sha256: string; parent_id?: string; recipe?: Generation['recipe'] }): Promise<Generation>;
export function rejectGeneration(id: string): Promise<Generation>;
export function listEvals(): Promise<{ evals: EvalRun[] }>;
export function recordEval(params: { candidate_id: string; baseline_id?: string; suite: string; verdict: EvalRun['verdict'] }): Promise<EvalRun>;
export function listPromotions(): Promise<{ promotions: Promotion[] }>;
export function promoteGeneration(params: { from_id: string; to_id: string; actor?: string }): Promise<Promotion>;
```

---

## 8. WebSocket Subscriptions in React

### Challenge

Old `ui/` uses imperative `subscribeRun()` from `sse.ts` with callbacks. React requires declarative subscription management with cleanup.

### Solution: Custom Hook with Effect

```typescript
// src/hooks/useRunSubscription.ts
import { useEffect, useRef, useCallback } from 'react';
import { subscribeRun, type EventEnvelope } from '../lib/sse';
import { useAppStore } from '../store/globalStore';

export function useRunSubscription(runId: string | null) {
  const subscriptionRef = useRef<ReturnType<typeof subscribeRun> | null>(null);
  const { actions } = useAppStore.getState();

  const handleEvent = useCallback((event: EventEnvelope) => {
    switch (event.type) {
      case 'model.delta':
        actions.updateStreamingText(String(event.payload.delta ?? ''));
        break;
      case 'model.reasoning_delta':
        actions.updateStreamingReasoning(String(event.payload.delta ?? ''));
        break;
      case 'tool.requested':
        actions.addToolCallFromEvent(event);
        break;
      case 'tool.started':
        actions.updateToolCallStatus(event, 'running');
        break;
      case 'tool.finished':
        actions.updateToolCallResult(event);
        break;
      case 'user.question_required':
        actions.setPendingQuestion({
          id: String(event.payload.question_id ?? ''),
          run_id: event.run_id,
          tool_call_id: String(event.payload.tool_call_id ?? ''),
          prompt: String(event.payload.prompt ?? ''),
          expires_at: Number(event.payload.expires_at ?? 0),
        });
        break;
      case 'run.completed':
      case 'run.failed':
      case 'run.cancelled':
        actions.finalizeRun(event.type.replace('run.', '') as 'completed' | 'failed' | 'cancelled');
        break;
      // Handle other event types...
    }
  }, [actions]);

  const handleError = useCallback((message: string) => {
    actions.setConnectionState('reconnecting');
    actions.setRunError(message);
  }, [actions]);

  useEffect(() => {
    if (!runId) {
      // Clean up previous subscription
      subscriptionRef.current?.close();
      subscriptionRef.current = null;
      return;
    }

    // Close existing subscription before starting new one
    subscriptionRef.current?.close();

    // Start new subscription
    subscriptionRef.current = subscribeRun(
      runId,
      0, // afterSeq: start from beginning
      handleEvent,
      handleError
    );

    actions.setConnectionState('connecting');

    // Cleanup on unmount or runId change
    return () => {
      subscriptionRef.current?.close();
      subscriptionRef.current = null;
    };
  }, [runId, handleEvent, handleError, actions]);

  // Expose manual close if needed
  const close = useCallback(() => {
    subscriptionRef.current?.close();
    subscriptionRef.current = null;
  }, []);

  return { close };
}
```

### Usage in Chat Hook

```typescript
// src/hooks/useChat.ts
import { useState, useCallback } from 'react';
import { useAppStore } from '../store/globalStore';
import { api } from '../lib/api';
import { useRunSubscription } from './useRunSubscription';

export function useChat() {
  const { currentSessionId, currentRun, actions } = useAppStore();
  const [sendPhase, setSendPhase] = useState<'idle' | 'processing' | 'error'>('idle');
  const [sendError, setSendError] = useState<string>('');

  // Auto-subscribe when currentRun changes
  useRunSubscription(currentRun?.id ?? null);

  const sendMessage = useCallback(async (text: string, mode: 'normal' | 'plan') => {
    if (!currentSessionId || !text || sendPhase === 'processing') return;

    setSendPhase('processing');
    setSendError('');

    try {
      // Preflight check
      const preflight = await api.preflight(currentSessionId, text, mode);
      if (preflight.status === 'blocked') {
        setSendError(`Blocked: ${preflight.blockers?.join('; ')}`);
        setSendPhase('error');
        return;
      }

      // Start run
      const { run_id, status } = await api.postMessage(currentSessionId, text, mode);
      
      // Update store
      actions.setCurrentRun({
        id: run_id,
        session_id: currentSessionId,
        status,
        created_at: Date.now(),
      });
      actions.addUserMessage(text);
      actions.clearDraft();
      
      // Subscription starts automatically via useRunSubscription effect
    } catch (error) {
      setSendError(error instanceof Error ? error.message : 'Failed to send message');
      setSendPhase('error');
    } finally {
      if (sendPhase === 'processing') {
        setSendPhase('idle');
      }
    }
  }, [currentSessionId, sendPhase, actions]);

  const cancelRun = useCallback(async () => {
    if (!currentRun) return;
    try {
      await api.cancelRun(currentRun.id);
    } catch (error) {
      console.error('Failed to cancel run:', error);
    }
  }, [currentRun]);

  return {
    sendMessage,
    cancelRun,
    sendPhase,
    sendError,
  };
}
```

### Key Points

1. **Effect dependency on `runId`**: Subscription restarts when run changes
2. **Cleanup function**: Closes subscription on unmount or runId change
3. **Callback stability**: `useCallback` prevents unnecessary effect re-runs
4. **Store integration**: Event handlers update Zustand store directly
5. **Auto-cleanup on terminal events**: `sse.ts` already closes subscription on `run.completed/failed/cancelled`

---

## 9. Demo Data Isolation Strategy

### Principle

All demo/local simulation data uses **centralized storage layer** with `vivy.demo.*` localStorage key prefix, completely isolated from real backend data.

### Implementation: `lib/demo-store.ts`

```typescript
// src/lib/demo-store.ts

const DEMO_PREFIX = 'vivy.demo.';

function getKey(category: string, key: string): string {
  return `${DEMO_PREFIX}${category}.${key}`;
}

function get<T>(category: string, key: string, defaultValue: T): T {
  try {
    const value = localStorage.getItem(getKey(category, key));
    return value ? JSON.parse(value) : defaultValue;
  } catch {
    return defaultValue;
  }
}

function set<T>(category: string, key: string, value: T): void {
  localStorage.setItem(getKey(category, key), JSON.stringify(value));
}

function remove(category: string, key: string): void {
  localStorage.removeItem(getKey(category, key));
}

// Persona demo storage
export const demoPersona = {
  getDocuments: () => get('persona', 'documents', {}),
  saveDocument: (kind: string, doc: any) => set('persona', `doc.${kind}`, doc),
  getHistory: (kind: string) => get('persona', `history.${kind}`, []),
  addHistoryEntry: (kind: string, entry: any) => {
    const history = get('persona', `history.${kind}`, []);
    set('persona', `history.${kind}`, [entry, ...history]);
  },
};

// Notebook demo storage
export const demoNotebook = {
  getReports: () => get('notebook', 'reports', []),
  addReport: (report: any) => {
    const reports = get('notebook', 'reports', []);
    set('notebook', 'reports', [report, ...reports]);
  },
  getSearchHits: () => get('notebook', 'searchHits', []),
};

// Cron demo storage
export const demoCron = {
  getJobs: () => get('cron', 'jobs', []),
  saveJobs: (jobs: any[]) => set('cron', 'jobs', jobs),
  addJob: (job: any) => {
    const jobs = get('cron', 'jobs', []);
    set('cron', 'jobs', [...jobs, job]);
  },
  updateJob: (id: string, updates: any) => {
    const jobs = get('cron', 'jobs', []);
    const updated = jobs.map((j: any) => j.id === id ? { ...j, ...updates } : j);
    set('cron', 'jobs', updated);
  },
  deleteJob: (id: string) => {
    const jobs = get('cron', 'jobs', []);
    set('cron', 'jobs', jobs.filter((j: any) => j.id !== id));
  },
};

// Clear all demo data (for testing/reset)
export function clearAllDemoData(): void {
  Object.keys(localStorage).forEach(key => {
    if (key.startsWith(DEMO_PREFIX)) {
      localStorage.removeItem(key);
    }
  });
}
```

### Hook Usage Example

```typescript
// src/hooks/useDemoPersona.ts
import { useState, useCallback } from 'react';
import { demoPersona } from '../lib/demo-store';
import type { PersonaKind, PersonaDocument } from '../lib/types';

export function useDemoPersona() {
  const [documents, setDocuments] = useState<Record<PersonaKind, PersonaDocument>>(() => 
    demoPersona.getDocuments()
  );

  const saveDocument = useCallback((kind: PersonaKind, content: string) => {
    const doc: PersonaDocument = {
      kind,
      content,
      revision: (documents[kind]?.revision ?? 0) + 1,
      updated_at: new Date().toISOString(),
    };
    
    demoPersona.saveDocument(kind, doc);
    demoPersona.addHistoryEntry(kind, {
      revision: doc.revision,
      updated_at: doc.updated_at,
      content_hash: `hash-${doc.revision}`,
    });
    
    setDocuments(prev => ({ ...prev, [kind]: doc }));
  }, [documents]);

  return {
    documents,
    saveDocument,
  };
}
```

### Visual Indicator

All demo pages must include `DemoBanner` component:

```tsx
// src/components/demo/DemoBanner.tsx
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { InfoIcon } from 'lucide-react';

export function DemoBanner() {
  return (
    <Alert variant="default" className="mb-4 border-yellow-500 bg-yellow-50 dark:bg-yellow-950">
      <InfoIcon className="h-4 w-4 text-yellow-600 dark:text-yellow-400" />
      <AlertTitle className="text-yellow-800 dark:text-yellow-200">
        Demo / Local Simulation
      </AlertTitle>
      <AlertDescription className="text-yellow-700 dark:text-yellow-300">
        This feature uses local browser storage for demonstration purposes. 
        Data is not synced with the backend and will be lost if you clear browser data.
      </AlertDescription>
    </Alert>
  );
}
```

Usage in demo pages:

```tsx
// src/routes/_layout.persona.tsx
import { DemoBanner } from '@/components/demo/DemoBanner';
import { PersonaMemoryView } from '@/components/demo/PersonaMemoryView';

export default function PersonaPage() {
  return (
    <div className="p-6">
      <DemoBanner />
      <PersonaMemoryView />
    </div>
  );
}
```

### localStorage Key Convention

| Category | Key Prefix | Example Keys |
|----------|-----------|--------------|
| Persona | `vivy.demo.persona.*` | `vivy.demo.persona.documents`, `vivy.demo.persona.history.identity` |
| Notebook | `vivy.demo.notebook.*` | `vivy.demo.notebook.reports`, `vivy.demo.notebook.searchHits` |
| Cron | `vivy.demo.cron.*` | `vivy.demo.cron.jobs` |

**Never mix** demo keys with real backend data. Real data comes from WebSocket/RPC calls only.

---

## 10. Testing Strategy Outline

### Unit Tests (Vitest + React Testing Library)

**Priority: Medium** (implement after core functionality works)

```typescript
// tests/unit/hooks/useSession.test.ts
import { describe, it, expect, vi } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useSession } from '@/hooks/useSession';

// Mock API
vi.mock('@/lib/api', () => ({
  listSessions: vi.fn(() => Promise.resolve({ sessions: [] })),
  createSession: vi.fn(() => Promise.resolve({ id: 'test', title: 'Test' })),
}));

describe('useSession', () => {
  it('loads sessions on mount', async () => {
    const { result } = renderHook(() => useSession());
    // Wait for async
    await vi.waitFor(() => {
      expect(result.current.sessions).toBeDefined();
    });
  });

  it('creates new session', async () => {
    const { result } = renderHook(() => useSession());
    await result.current.createSession('New Session');
    expect(result.current.sessions).toHaveLength(1);
  });
});
```

**Test Coverage Targets:**
- Hooks: `useSession`, `useChat`, `useApprovals`, `useSettings`
- Components: `MessageBubble`, `ToolCallRenderer`, `ApprovalCard`
- Utilities: `demo-store.ts` functions

### Integration Tests (Playwright)

**Priority: High** (critical for real backend interaction)

```typescript
// tests/e2e/chat.spec.ts
import { test, expect } from '@playwright/test';

test('send message and receive streaming response', async ({ page }) => {
  await page.goto('/');
  
  // Type message
  await page.getByTestId('chat-input').fill('Hello');
  await page.getByTestId('send-button').click();
  
  // Wait for user message to appear
  await expect(page.getByText('Hello')).toBeVisible();
  
  // Wait for streaming response
  await expect(page.getByTestId('streaming-text')).toBeVisible();
  
  // Wait for completion
  await expect(page.getByTestId('run-status')).toHaveText('completed');
});

test('approve tool call', async ({ page }) => {
  await page.goto('/approvals');
  
  // Wait for pending approval
  await expect(page.getByText('Approval Required')).toBeVisible();
  
  // Click approve
  await page.getByTestId('approve-button').click();
  
  // Verify approval resolved
  await expect(page.getByText('Approved')).toBeVisible();
});
```

**E2E Test Scenarios:**
1. Session CRUD (create, rename, delete, switch)
2. Chat flow (send message, stream response, cancel run)
3. Approval workflow (list, approve/deny, question answer)
4. Settings update (change provider, model, base URL)
5. Demo features (persona edit, notebook report generation, cron job creation)

### Manual Testing Checklist

Before release, verify:

- [ ] WebSocket connects successfully on app load
- [ ] Session list loads from backend
- [ ] Sending message triggers preflight → postMessage → subscription
- [ ] Streaming text updates in real-time
- [ ] Tool calls display with correct status (running → success/error)
- [ ] AskUserQuestion modal appears and can be answered
- [ ] Approval cards show in Review Center
- [ ] Settings save persists to backend
- [ ] Theme/locale toggles work
- [ ] Demo pages show `DemoBanner`
- [ ] Demo data uses `vivy.demo.*` keys only
- [ ] No references to mock API remain
- [ ] No dead routes (/pet, /dashboard, /memory, /mcp)
- [ ] Branding consistently says "Vivy"

### Performance Benchmarks

- Initial load: < 2 seconds (with cached assets)
- Session list load: < 500ms
- Message send → first delta: < 1 second
- Re-render count per keystroke: 0 (controlled input)
- Re-render per streaming delta: 1 (only MessageBubble updates)

---

## Appendix A: Dependency List

### Production Dependencies

```json
{
  "dependencies": {
    "@tanstack/react-query": "^5.x",
    "@tanstack/react-router": "^1.x",
    "react": "^18.x",
    "react-dom": "^18.x",
    "zustand": "^4.x",
    "lucide-react": "^0.x",
    "class-variance-authority": "^0.x",
    "clsx": "^2.x",
    "tailwind-merge": "^2.x",
    "remarkable": "^2.x"
  }
}
```

### Dev Dependencies

```json
{
  "devDependencies": {
    "@tanstack/router-plugin": "^1.x",
    "@types/react": "^18.x",
    "@types/react-dom": "^18.x",
    "@vitejs/plugin-react": "^4.x",
    "autoprefixer": "^10.x",
    "postcss": "^8.x",
    "tailwindcss": "^4.x",
    "typescript": "^5.x",
    "vite": "^5.x",
    "vitest": "^1.x",
    "@testing-library/react": "^14.x",
    "@playwright/test": "^1.x"
  }
}
```

### shadcn/ui Components Used

All components from `new-ui/src/components/ui/` are copied as-is:
- button, input, textarea, select, tabs, badge, card, scroll-area, skeleton, tooltip, dropdown-menu, sheet, sidebar, dialog, alert, separator, label, switch, checkbox, radio-group, slider, progress, avatar, breadcrumb, calendar, carousel, chart, collapsible, command, context-menu, drawer, form, hover-card, input-otp, menubar, navigation-menu, pagination, popover, resizable, sonner, toggle, toggle-group

---

## Appendix B: Migration Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| WebSocket reconnection logic breaks | High | Preserve exact `sse.ts` implementation; write E2E test for disconnect/reconnect |
| State duplication between hooks | Medium | Enforce single global store; audit all hooks to ensure they read/write store, not local state |
| Demo data leaks into real API | Medium | Code review all demo hooks to ensure they only use `demo-store.ts`; add lint rule to forbid `localStorage` usage outside `demo-store.ts` |
| Branding inconsistencies | Low | Search-and-replace "Agent-Diva"/"DiVA"/"Meoo" → "Vivy"; verify in all UI strings |
| Missing i18n for new strings | Low | Extract all hardcoded strings to i18n files; use old `ui/src/app/i18n.ts` as base |
| TanStack Router route conflicts | Medium | Follow naming convention strictly (`_layout.xxx.tsx`); regenerate `routeTree.gen.ts` after each route addition |
| shadcn/ui theme incompatibility | Low | Copy Tailwind config from new-ui; verify dark mode works |

---

## Appendix C: Glossary

- **RPC**: Remote Procedure Call via WebSocket JSON-RPC 2.0
- **SSE**: Server-Sent Events (implemented as WebSocket notifications in Vivy)
- **Preflight**: Safety check before starting a run (policy evaluation, tool decisions)
- **Run**: Single execution of agent loop (can be active, completed, failed, cancelled)
- **Child Run**: Sub-run spawned by parent run for parallel/delegated work
- **Background Run**: Long-running run detached from UI session
- **Approval**: Human-in-the-loop authorization for risky tool calls
- **AskUserQuestion**: Agent requests clarification from user during run
- **Review Center**: Unified view of pending approvals and questions
- **Studio**: Vivy's self-evolution interface (generations, evals, promotions)
- **Demo Data**: Local-only simulation data stored in `vivy.demo.*` localStorage keys

---

**End of Document**

This architecture document provides sufficient detail for another agent to implement the unified Vivy React UI. Key implementation steps:

1. Set up directory structure and dependencies
2. Create global store (Zustand)
3. Migrate infrastructure files (`rpc.ts`, `sse.ts`, `api.ts`)
4. Build hooks layer wrapping API calls
5. Adapt components from new-ui to use real hooks
6. Implement routing with TanStack Router
7. Add demo isolation layer
8. Clean up branding and remove dead code
9. Test thoroughly with E2E tests
