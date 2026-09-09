# UI Migration - Styling System Extension Plan

## Overview

This document plans how to adapt Agent Diva's rich styling (TailwindCSS + custom variables) to VIVY's streamlined design-token system.

### Current-State Comparison

| Item | Agent Diva | VIVY | Gap |
|------|-----------|------|------|
| CSS file size | styles.css: 4339 lines | tokens.css: 119 lines | VIVY lacks many style definitions |
| Tech stack | TailwindCSS + custom variables | CSS variables only | ⚠️ Adaptation required |
| Number of themes | 3+ (Love/Dark/Default) | 2 (Light/Dark + System) | ✅ Consistent |
| Design language | Pink gradients/Glassmorphism | Neutral colors/industrial style | ⚠️ Major style differences |
| Component library | Complete (50+ components) | Basic (~10 elements) | ❌ Many component styles missing |

---

## 1. Agent Diva Styling Analysis

### 1.1 Core Design Tokens

Key variable categories extracted from `styles.css`:

#### Semantic Color System
```css
/* Agent Diva */
--danger: #ef4444;
--danger-bg: rgba(239, 68, 68, 0.1);
--success: #22c55e / #34d399;
--success-bg: rgba(34, 197, 94, 0.1);
--warning: #f59e0b / #fbbf24;
--warning-bg: rgba(245, 158, 11, 0.1);
--info: #3b82f6 / #60a5fa;
--info-bg: rgba(59, 130, 246, 0.1);
```

**Already in VIVY:** ✅ Fully covered
```css
--danger: #d6455b;
--danger-soft: #fdecef;
--success: #2f9e6b;
--success-soft: #e6f6ee;
--warning: #c77d1e;
--warning-soft: #fbf0dc;
```

#### Surface Hierarchy
```css
/* Agent Diva */
--surface-raised: rgba(255, 255, 255, 0.72);
--surface-sunken: rgba(236, 72, 153, 0.05);
--overlay: rgba(107, 39, 55, 0.35);
```

**Already in VIVY:** ✅ Partially covered
```css
--surface-canvas: #f5f6f8;
--surface-panel: #ffffff;
--surface-subtle: #eef0f3;
--surface-raised: #ffffff;
--surface-selected: #e9e9ff;
```

**Gap:** `--surface-sunken`, `--overlay`

#### Chat-Bubble Variables
```css
/* Agent Diva */
--bubble-user-radius: 18px 18px 4px 18px;
--bubble-assistant-radius: 18px 18px 18px 4px;
--bubble-shadow: 0 1px 3px rgba(0, 0, 0, 0.08), 0 4px 12px rgba(0, 0, 0, 0.05);
--bubble-glow: 0 4px 16px rgba(236, 72, 153, 0.2);
--bubble-padding: 12px 16px;
--bubble-max-width: 85%;
```

**Already in VIVY:** ❌ Completely missing

#### Navigation and Sidebar
```css
/* Agent Diva */
--sidebar-width: 260px;
--sidebar-collapsed-width: 56px;
--nav-hover: rgba(236, 72, 153, 0.08);
--nav-active: rgba(236, 72, 153, 0.12);
```

**Already in VIVY:** ⚠️ Partial
```css
--rail-w: 264px;  /* ≈ sidebar-width */
```

**Gap:** collapsed state and navigation hover/active states

### 1.2 Component-Style Inventory

Agent Diva's main components and their styling requirements:

| Component | Estimated Lines | Complexity | Priority |
|------|---------|--------|--------|
| ChatView | ~800 | High | 🔴 P0 |
| ConversationSidebar | ~400 | Medium | 🔴 P0 |
| SettingsView | ~600 | High | 🟡 P1 |
| ApprovalCenterDrawer | ~300 | Medium | 🟡 P1 |
| PlanApprovalCard | ~250 | Medium | 🟡 P1 |
| TodoCard/TodoList | ~200 | Low | 🟢 P2 |
| ConsoleView | ~350 | Medium | 🟢 P2 |
| NotebookView | ~300 | Medium | 🟢 P2 |
| PersonaMemoryView | ~250 | Medium | 🟢 P2 |
| EvolutionView | ~200 | Low | 🟢 P3 |
| SkillsSettings | ~250 | Low | 🟢 P3 |
| McpSettings | ~200 | Low | 🟢 P3 |

---

## 2. Migration Strategy

### 2.1 Design-Principle Alignment

**Problem:** Agent Diva uses pink gradients and Glassmorphism, while VIVY uses a neutral industrial style.

**Decision:** 
- **Retain VIVY's design language** (consistent with the DeepSeek Harness IDE style)
- **Reuse Agent Diva's layout structure and interaction patterns**
- **Adjust colors to match VIVY's palette**

**Example:**
```css
/* Agent Diva (pink gradient bubbles) */
--bubble-user-bg: linear-gradient(135deg, #ffd3e1 0%, #ffb4cc 100%);

/* VIVY adaptation (keep neutral colors) */
--bubble-user-bg: var(--primary-soft);  /* use the theme's primary color */
```

### 2.2 CSS Variable Extension Inventory

New variables to add to `tokens.css`:

#### A. Chat-Bubble System (P0)
```css
:root {
  /* bubble geometry */
  --bubble-radius-user: 18px 18px 4px 18px;
  --bubble-radius-assistant: 18px 18px 18px 4px;
  --bubble-padding: 12px 16px;
  --bubble-max-width: 85%;
  
  /* bubble shadow */
  --bubble-shadow-sm: 0 1px 3px rgb(20 22 30 / 0.08), 0 4px 12px rgb(20 22 30 / 0.05);
  --bubble-glow: 0 4px 16px var(--primary-soft);
  
  /* bubble background (theme-dependent) */
  --bubble-user-bg: var(--primary);
  --bubble-user-text: #ffffff;
  --bubble-assistant-bg: var(--surface-panel);
  --bubble-assistant-text: var(--text-strong);
}

:root[data-theme="dark"] {
  --bubble-shadow-sm: 0 1px 3px rgb(0 0 0 / 0.3), 0 4px 12px rgb(0 0 0 / 0.2);
  --bubble-user-bg: var(--primary);
  --bubble-user-text: #ffffff;
  --bubble-assistant-bg: var(--surface-panel);
  --bubble-assistant-text: var(--text-strong);
}
```

#### B. Navigation and Sidebar Enhancements (P0)
```css
:root {
  /* sidebar states */
  --sidebar-collapsed-w: 56px;
  
  /* navigation interactions */
  --nav-item-hover-bg: var(--surface-subtle);
  --nav-item-active-bg: var(--surface-selected);
  --nav-item-active-indicator: var(--primary);
}
```

#### C. Card and Panel System (P1)
```css
:root {
  /* card variants */
  --card-bg: var(--surface-panel);
  --card-border: var(--border);
  --card-shadow: var(--shadow-sm);
  --card-radius: var(--radius-md);
  
  /* clickable card hover */
  --card-hover-bg: var(--surface-subtle);
  --card-hover-border: var(--border-strong);
  
  /* embedded panels (such as approval cards) */
  --panel-embedded-bg: var(--surface-subtle);
  --panel-embedded-border: var(--border);
  --panel-embedded-radius: var(--radius-sm);
}
```

#### D. Form Controls (P1)
```css
:root {
  /* input fields */
  --input-bg: var(--surface-panel);
  --input-border: var(--border);
  --input-focus-border: var(--primary);
  --input-focus-ring: 0 0 0 3px var(--primary-soft);
  --input-placeholder: var(--text-faint);
  
  /* button variants */
  --button-primary-bg: var(--primary);
  --button-primary-text: #ffffff;
  --button-secondary-bg: var(--surface-subtle);
  --button-secondary-text: var(--text-strong);
  --button-danger-bg: var(--danger);
  --button-danger-text: #ffffff;
  
  /* button states */
  --button-hover-opacity: 0.9;
  --button-disabled-opacity: 0.5;
}
```

#### E. Badges and Status Indicators (P1)
```css
:root {
  /* status badges */
  --badge-bg: var(--surface-subtle);
  --badge-text: var(--text-muted);
  --badge-radius: var(--radius-pill);
  
  /* semantic badges */
  --badge-success-bg: var(--success-soft);
  --badge-success-text: var(--success);
  --badge-warning-bg: var(--warning-soft);
  --badge-warning-text: var(--warning);
  --badge-danger-bg: var(--danger-soft);
  --badge-danger-text: var(--danger);
  
  /* connection status dot */
  --status-dot-size: 8px;
  --status-online: var(--success);
  --status-offline: var(--text-faint);
  --status-connecting: var(--warning);
}
```

#### F. Code and Terminal (P2)
```css
:root {
  /* inline code */
  --code-inline-bg: var(--surface-subtle);
  --code-inline-text: var(--text-strong);
  --code-inline-radius: var(--radius-sm);
  
  /* code blocks */
  --code-block-bg: var(--surface-canvas);
  --code-block-border: var(--border);
  --code-block-header-bg: var(--surface-subtle);
  
  /* terminal output */
  --terminal-bg: var(--surface-canvas);
  --terminal-text: var(--text-strong);
  --terminal-prompt: var(--text-muted);
}
```

#### G. Loading and Progress (P2)
```css
:root {
  /* Spinner */
  --spinner-size: 20px;
  --spinner-color: var(--primary);
  
  /* progress bars */
  --progress-height: 4px;
  --progress-bg: var(--surface-subtle);
  --progress-fill: var(--primary);
  --progress-radius: var(--radius-pill);
  
  /* skeleton loading */
  --skeleton-bg: var(--surface-subtle);
  --skeleton-animation: pulse 1.5s ease-in-out infinite;
}
```

#### H. Modals and Overlays (P2)
```css
:root {
  /* scrim overlay */
  --overlay-bg: rgba(20 22 30 / 0.5);
  --overlay-blur: blur(4px);
  
  /* modal dialogs */
  --modal-bg: var(--surface-raised);
  --modal-border: var(--border-strong);
  --modal-shadow: var(--shadow-lg);
  --modal-radius: var(--radius-lg);
  --modal-max-width: 600px;
  --modal-max-height: 80vh;
}

:root[data-theme="dark"] {
  --overlay-bg: rgba(0 0 0 / 0.6);
}
```

#### I. Toast Notifications (P2)
```css
:root {
  --toast-bg: var(--surface-raised);
  --toast-border: var(--border-strong);
  --toast-shadow: var(--shadow-lg);
  --toast-radius: var(--radius-md);
  
  /* toast variants */
  --toast-success-border: var(--success);
  --toast-error-border: var(--danger);
  --toast-warning-border: var(--warning);
}
```

#### J. Tooltips (P3)
```css
:root {
  --tooltip-bg: var(--surface-raised);
  --tooltip-text: var(--text-strong);
  --tooltip-border: var(--border);
  --tooltip-shadow: var(--shadow-sm);
  --tooltip-radius: var(--radius-sm);
}
```

---

## 3. Implementation Steps

### Step 1: Extend tokens.css (1 Day)

**Tasks:**
1. Add the new variable groups to the end of the existing `tokens.css` (using the A-J categories above)
2. Ensure that Light/Dark/System modes all have corresponding values
3. Run browser tests to verify correct variable inheritance

**Example Code Structure:**
```css
/* === existing content remains unchanged === */
:root { ... }
:root[data-theme="dark"] { ... }
@media (prefers-color-scheme: dark) { ... }

/* === added: chat bubble system === */
:root {
  --bubble-radius-user: 18px 18px 4px 18px;
  --bubble-radius-assistant: 18px 18px 18px 4px;
  /* ... */
}

:root[data-theme="dark"] {
  /* dark-mode overrides */
}

/* === added: navigation enhancements === */
:root {
  --sidebar-collapsed-w: 56px;
  /* ... */
}

/* ... other added groups */
```

### Step 2: Create Component Style Files (2-3 Days)

**Strategy:** Do not use Tailwind; instead, create a separate CSS file for each major component and use semantic class names.

**File Structure:**
```
ui/src/styles/
├── tokens.css          # design tokens (existing)
├── base.css            # base reset (existing)
├── layout.css          # layout grid (existing?)
├── components/
│   ├── chat.css        # ChatView styles
│   ├── sidebar.css     # ConversationSidebar styles
│   ├── settings.css    # SettingsView styles
│   ├── approval.css    # ApprovalCenter styles
│   ├── plan.css        # Plan-related card styles
│   ├── console.css     # ConsoleView styles
│   └── shared/
│       ├── card.css    # shared card styles
│       ├── button.css  # button variants
│       ├── input.css   # form controls
│       └── badge.css   # badges and states
```

**Example: `components/chat.css`**
```css
/* chat area container */
.chat-region {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

/* message list */
.message-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* message bubble base class */
.message-bubble {
  max-width: var(--bubble-max-width);
  padding: var(--bubble-padding);
  border-radius: var(--radius-md);
  box-shadow: var(--bubble-shadow-sm);
}

/* user message */
.message-bubble.user {
  align-self: flex-end;
  background: var(--bubble-user-bg);
  color: var(--bubble-user-text);
  border-radius: var(--bubble-radius-user);
}

/* assistant message */
.message-bubble.assistant {
  align-self: flex-start;
  background: var(--bubble-assistant-bg);
  color: var(--bubble-assistant-text);
  border-radius: var(--bubble-radius-assistant);
}

/* Composer input area */
.composer {
  padding: var(--space-4) var(--space-6);
  border-top: 1px solid var(--border);
  background: var(--surface-panel);
}

.composer-input-wrap textarea {
  width: 100%;
  min-height: 60px;
  max-height: 200px;
  padding: var(--space-3);
  border: 1px solid var(--input-border);
  border-radius: var(--radius-sm);
  background: var(--input-bg);
  color: var(--text-strong);
  resize: vertical;
}

.composer-input-wrap textarea:focus {
  outline: none;
  border-color: var(--input-focus-border);
  box-shadow: var(--input-focus-ring);
}
```

### Step 3: Import the New Style Files (Half Day)

**Modify `index.html` or `main.ts`:**
```typescript
// ui/src/main.ts
import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/layout.css";

// added component styles
import "./styles/components/chat.css";
import "./styles/components/sidebar.css";
import "./styles/components/settings.css";
import "./styles/components/approval.css";
import "./styles/components/plan.css";
import "./styles/components/console.css";
import "./styles/components/shared/card.css";
import "./styles/components/shared/button.css";
import "./styles/components/shared/input.css";
import "./styles/components/shared/badge.css";
```

### Step 4: Remove the Tailwind Dependency (1 Day)

**Problem:** Agent Diva relies heavily on Tailwind utility classes (such as `flex items-center gap-2 p-4 bg-white rounded-lg shadow`).

**Option A: Manually Convert to Semantic CSS**
```html
<!-- Before (Tailwind) -->
<div class="flex items-center gap-2 p-4 bg-white rounded-lg shadow">

<!-- After (Semantic CSS) -->
<div class="card compact">
```

```css
.card {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-4);
  background: var(--surface-panel);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
}

.card.compact {
  gap: var(--space-2);
  padding: var(--space-2);
}
```

**Option B: Keep the Tailwind CDN (Not Recommended)**
- Pros: Faster migration
- Cons: Adds a runtime dependency and conflicts with VIVY's philosophy

**Decision: Option A**

### Step 5: Visual Regression Testing (1-2 Days)

**Tasks:**
1. Compare screenshots of key pages (chat, settings, Approval Center)
2. Ensure Light/Dark modes switch correctly
3. Check responsive layouts (mobile support)

**Tool:** Playwright screenshot comparison

---

## 4. Effort Estimate

| Task | Effort | Owner |
|------|------|--------|
| Step 1: Extend tokens.css | 1 day | Frontend |
| Step 2: Create component style files | 2-3 days | Frontend |
| Step 3: Import the new style files | 0.5 day | Frontend |
| Step 4: Remove the Tailwind dependency | 1 day | Frontend |
| Step 5: Visual regression testing | 1-2 days | QA |
| **Total** | **5.5-7.5 days** | |

---

## 5. Acceptance Criteria

- [ ] All new CSS variables display correctly in Light/Dark modes
- [ ] Core component styles such as chat bubbles, the sidebar, and the settings panel are complete
- [ ] No Tailwind class names remain (confirmed with grep)
- [ ] Playwright visual regression tests pass (deviation < 5%)
- [ ] Build output size increases by < 50KB (gzipped)

---

## 6. Risks and Mitigation

| Risk | Impact | Mitigation |
|------|------|---------|
| Manual Tailwind conversion is labor-intensive | Schedule delay | Prioritize P0/P1 components and defer P2/P3 |
| Inconsistent styles cause visual confusion | Degraded user experience | Follow VIVY design tokens strictly; do not introduce creative deviations |
| Responsive layouts are not adapted | Unusable on mobile | Use CSS Grid/Flexbox and avoid fixed widths |

---

**Document Version:** v0.1  
**Created:** 2026-01-XX  
**Maintainer:** UI Migration Team  
**Status:** Draft - pending review
