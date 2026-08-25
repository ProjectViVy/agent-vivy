# UI 迁移 - 样式系统扩展计划

## 概述

本文档规划如何将 Agent Diva 的丰富样式（TailwindCSS + 自定义变量）适配到 VIVY 的简约设计令牌系统中。

### 当前状态对比

| 项目 | Agent Diva | VIVY | 差距 |
|------|-----------|------|------|
| CSS 文件大小 | styles.css: 4339 行 | tokens.css: 119 行 | VIVY 缺少大量样式定义 |
| 技术栈 | TailwindCSS + 自定义变量 | 纯 CSS 变量 | ⚠️ 需要适配 |
| 主题数量 | 3+ (Love/Dark/Default) | 2 (Light/Dark + System) | ✅ 一致 |
| 设计语言 | 粉色渐变/Glassmorphism | 中性色/工业风 | ⚠️ 风格差异大 |
| 组件库 | 完整（50+ 组件） | 基础（~10 个元素） | ❌ 缺失大量组件样式 |

---

## 1. Agent Diva 样式分析

### 1.1 核心设计令牌

从 `styles.css` 提取的关键变量类别：

#### 语义色系统
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

**VIVY 已有：** ✅ 完全覆盖
```css
--danger: #d6455b;
--danger-soft: #fdecef;
--success: #2f9e6b;
--success-soft: #e6f6ee;
--warning: #c77d1e;
--warning-soft: #fbf0dc;
```

#### 表面层级
```css
/* Agent Diva */
--surface-raised: rgba(255, 255, 255, 0.72);
--surface-sunken: rgba(236, 72, 153, 0.05);
--overlay: rgba(107, 39, 55, 0.35);
```

**VIVY 已有：** ✅ 部分覆盖
```css
--surface-canvas: #f5f6f8;
--surface-panel: #ffffff;
--surface-subtle: #eef0f3;
--surface-raised: #ffffff;
--surface-selected: #e9e9ff;
```

**缺口：** `--surface-sunken`, `--overlay`

#### 聊天气泡专用变量
```css
/* Agent Diva */
--bubble-user-radius: 18px 18px 4px 18px;
--bubble-assistant-radius: 18px 18px 18px 4px;
--bubble-shadow: 0 1px 3px rgba(0, 0, 0, 0.08), 0 4px 12px rgba(0, 0, 0, 0.05);
--bubble-glow: 0 4px 16px rgba(236, 72, 153, 0.2);
--bubble-padding: 12px 16px;
--bubble-max-width: 85%;
```

**VIVY 已有：** ❌ 完全缺失

#### 导航与侧边栏
```css
/* Agent Diva */
--sidebar-width: 260px;
--sidebar-collapsed-width: 56px;
--nav-hover: rgba(236, 72, 153, 0.08);
--nav-active: rgba(236, 72, 153, 0.12);
```

**VIVY 已有：** ⚠️ 部分
```css
--rail-w: 264px;  /* ≈ sidebar-width */
```

**缺口：** collapsed 状态、导航悬停/激活状态

### 1.2 组件样式清单

Agent Diva 的主要组件及其样式需求：

| 组件 | 行数估算 | 复杂度 | 优先级 |
|------|---------|--------|--------|
| ChatView | ~800 | 高 | 🔴 P0 |
| ConversationSidebar | ~400 | 中 | 🔴 P0 |
| SettingsView | ~600 | 高 | 🟡 P1 |
| ApprovalCenterDrawer | ~300 | 中 | 🟡 P1 |
| PlanApprovalCard | ~250 | 中 | 🟡 P1 |
| TodoCard/TodoList | ~200 | 低 | 🟢 P2 |
| ConsoleView | ~350 | 中 | 🟢 P2 |
| NotebookView | ~300 | 中 | 🟢 P2 |
| PersonaMemoryView | ~250 | 中 | 🟢 P2 |
| EvolutionView | ~200 | 低 | 🟢 P3 |
| SkillsSettings | ~250 | 低 | 🟢 P3 |
| McpSettings | ~200 | 低 | 🟢 P3 |

---

## 2. 迁移策略

### 2.1 设计原则对齐

**问题：** Agent Diva 使用粉色渐变和 Glassmorphism，VIVY 使用中性工业风。

**决策：** 
- **保留 VIVY 的设计语言**（符合 DeepSeek Harness 的 IDE 风格）
- **复用 Agent Diva 的布局结构和交互模式**
- **调整颜色以匹配 VIVY 调色板**

**示例：**
```css
/* Agent Diva（粉色渐变气泡） */
--bubble-user-bg: linear-gradient(135deg, #ffd3e1 0%, #ffb4cc 100%);

/* VIVY 适配（保持中性色） */
--bubble-user-bg: var(--primary-soft);  /* 使用主题主色 */
```

### 2.2 CSS 变量扩展清单

需要在 `tokens.css` 中添加的新变量：

#### A. 聊天气泡系统（P0）
```css
:root {
  /* 气泡几何 */
  --bubble-radius-user: 18px 18px 4px 18px;
  --bubble-radius-assistant: 18px 18px 18px 4px;
  --bubble-padding: 12px 16px;
  --bubble-max-width: 85%;
  
  /* 气泡阴影 */
  --bubble-shadow-sm: 0 1px 3px rgb(20 22 30 / 0.08), 0 4px 12px rgb(20 22 30 / 0.05);
  --bubble-glow: 0 4px 16px var(--primary-soft);
  
  /* 气泡背景（由主题决定） */
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

#### B. 导航与侧边栏增强（P0）
```css
:root {
  /* 侧边栏状态 */
  --sidebar-collapsed-w: 56px;
  
  /* 导航交互 */
  --nav-item-hover-bg: var(--surface-subtle);
  --nav-item-active-bg: var(--surface-selected);
  --nav-item-active-indicator: var(--primary);
}
```

#### C. 卡片与面板系统（P1）
```css
:root {
  /* 卡片变体 */
  --card-bg: var(--surface-panel);
  --card-border: var(--border);
  --card-shadow: var(--shadow-sm);
  --card-radius: var(--radius-md);
  
  /* 可点击卡片悬停 */
  --card-hover-bg: var(--surface-subtle);
  --card-hover-border: var(--border-strong);
  
  /* 嵌入式面板（如审批卡片） */
  --panel-embedded-bg: var(--surface-subtle);
  --panel-embedded-border: var(--border);
  --panel-embedded-radius: var(--radius-sm);
}
```

#### D. 表单控件（P1）
```css
:root {
  /* 输入框 */
  --input-bg: var(--surface-panel);
  --input-border: var(--border);
  --input-focus-border: var(--primary);
  --input-focus-ring: 0 0 0 3px var(--primary-soft);
  --input-placeholder: var(--text-faint);
  
  /* 按钮变体 */
  --button-primary-bg: var(--primary);
  --button-primary-text: #ffffff;
  --button-secondary-bg: var(--surface-subtle);
  --button-secondary-text: var(--text-strong);
  --button-danger-bg: var(--danger);
  --button-danger-text: #ffffff;
  
  /* 按钮状态 */
  --button-hover-opacity: 0.9;
  --button-disabled-opacity: 0.5;
}
```

#### E. 徽章与状态指示器（P1）
```css
:root {
  /* 状态徽章 */
  --badge-bg: var(--surface-subtle);
  --badge-text: var(--text-muted);
  --badge-radius: var(--radius-pill);
  
  /* 语义徽章 */
  --badge-success-bg: var(--success-soft);
  --badge-success-text: var(--success);
  --badge-warning-bg: var(--warning-soft);
  --badge-warning-text: var(--warning);
  --badge-danger-bg: var(--danger-soft);
  --badge-danger-text: var(--danger);
  
  /* 连接状态点 */
  --status-dot-size: 8px;
  --status-online: var(--success);
  --status-offline: var(--text-faint);
  --status-connecting: var(--warning);
}
```

#### F. 代码与终端（P2）
```css
:root {
  /* 内联代码 */
  --code-inline-bg: var(--surface-subtle);
  --code-inline-text: var(--text-strong);
  --code-inline-radius: var(--radius-sm);
  
  /* 代码块 */
  --code-block-bg: var(--surface-canvas);
  --code-block-border: var(--border);
  --code-block-header-bg: var(--surface-subtle);
  
  /* 终端输出 */
  --terminal-bg: var(--surface-canvas);
  --terminal-text: var(--text-strong);
  --terminal-prompt: var(--text-muted);
}
```

#### G. 加载与进度（P2）
```css
:root {
  /* Spinner */
  --spinner-size: 20px;
  --spinner-color: var(--primary);
  
  /* 进度条 */
  --progress-height: 4px;
  --progress-bg: var(--surface-subtle);
  --progress-fill: var(--primary);
  --progress-radius: var(--radius-pill);
  
  /* Skeleton 加载 */
  --skeleton-bg: var(--surface-subtle);
  --skeleton-animation: pulse 1.5s ease-in-out infinite;
}
```

#### H. 模态框与遮罩（P2）
```css
:root {
  /* 遮罩层 */
  --overlay-bg: rgba(20 22 30 / 0.5);
  --overlay-blur: blur(4px);
  
  /* 模态框 */
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

#### I. Toast 通知（P2）
```css
:root {
  --toast-bg: var(--surface-raised);
  --toast-border: var(--border-strong);
  --toast-shadow: var(--shadow-lg);
  --toast-radius: var(--radius-md);
  
  /* Toast 变体 */
  --toast-success-border: var(--success);
  --toast-error-border: var(--danger);
  --toast-warning-border: var(--warning);
}
```

#### J. 工具提示（P3）
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

## 3. 实施步骤

### Step 1：扩展 tokens.css（1 天）

**任务：**
1. 在现有 `tokens.css` 末尾添加新变量组（按上述 A-J 分类）
2. 确保 Light/Dark/System 三种模式都有对应值
3. 运行浏览器测试，验证变量继承正确

**示例代码结构：**
```css
/* === 现有内容保持不变 === */
:root { ... }
:root[data-theme="dark"] { ... }
@media (prefers-color-scheme: dark) { ... }

/* === 新增：聊天气泡系统 === */
:root {
  --bubble-radius-user: 18px 18px 4px 18px;
  --bubble-radius-assistant: 18px 18px 18px 4px;
  /* ... */
}

:root[data-theme="dark"] {
  /* Dark 模式覆盖 */
}

/* === 新增：导航增强 === */
:root {
  --sidebar-collapsed-w: 56px;
  /* ... */
}

/* ... 其他新增组 */
```

### Step 2：创建组件样式文件（2-3 天）

**策略：** 不使用 Tailwind，而是为每个主要组件创建独立的 CSS 文件，使用语义化类名。

**文件结构：**
```
ui/src/styles/
├── tokens.css          # 设计令牌（已存在）
├── base.css            # 基础重置（已存在）
├── layout.css          # 布局网格（已存在？）
├── components/
│   ├── chat.css        # ChatView 样式
│   ├── sidebar.css     # ConversationSidebar 样式
│   ├── settings.css    # SettingsView 样式
│   ├── approval.css    # ApprovalCenter 样式
│   ├── plan.css        # Plan 相关卡片样式
│   ├── console.css     # ConsoleView 样式
│   └── shared/
│       ├── card.css    # 通用卡片样式
│       ├── button.css  # 按钮变体
│       ├── input.css   # 表单控件
│       └── badge.css   # 徽章与状态
```

**示例：`components/chat.css`**
```css
/* 聊天区域容器 */
.chat-region {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

/* 消息列表 */
.message-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* 消息气泡基类 */
.message-bubble {
  max-width: var(--bubble-max-width);
  padding: var(--bubble-padding);
  border-radius: var(--radius-md);
  box-shadow: var(--bubble-shadow-sm);
}

/* 用户消息 */
.message-bubble.user {
  align-self: flex-end;
  background: var(--bubble-user-bg);
  color: var(--bubble-user-text);
  border-radius: var(--bubble-radius-user);
}

/* 助手消息 */
.message-bubble.assistant {
  align-self: flex-start;
  background: var(--bubble-assistant-bg);
  color: var(--bubble-assistant-text);
  border-radius: var(--bubble-radius-assistant);
}

/* Composer 输入区 */
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

### Step 3：导入新样式文件（半天）

**修改 `index.html` 或 `main.ts`：**
```typescript
// ui/src/main.ts
import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/layout.css";

// 新增组件样式
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

### Step 4：移除 Tailwind 依赖（1 天）

**问题：** Agent Diva 重度依赖 Tailwind 工具类（如 `flex items-center gap-2 p-4 bg-white rounded-lg shadow`）。

**方案 A：手动转换为语义化 CSS**
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

**方案 B：保留 Tailwind CDN（不推荐）**
- 优点：快速迁移
- 缺点：增加运行时依赖，违背 VIVY 哲学

**决策：方案 A**

### Step 5：视觉回归测试（1-2 天）

**任务：**
1. 截图对比关键页面（聊天、设置、审批中心）
2. 确保 Light/Dark 模式正常切换
3. 检查响应式布局（移动端适配）

**工具：** Playwright 截图对比

---

## 4. 工作量估算

| 任务 | 工时 | 负责人 |
|------|------|--------|
| Step 1: 扩展 tokens.css | 1 天 | Frontend |
| Step 2: 创建组件样式文件 | 2-3 天 | Frontend |
| Step 3: 导入新样式文件 | 0.5 天 | Frontend |
| Step 4: 移除 Tailwind 依赖 | 1 天 | Frontend |
| Step 5: 视觉回归测试 | 1-2 天 | QA |
| **总计** | **5.5-7.5 天** | |

---

## 5. 验收标准

- [ ] 所有新 CSS 变量在 Light/Dark 模式下正确显示
- [ ] 聊天气泡、侧边栏、设置面板等核心组件样式完整
- [ ] 无 Tailwind 类名残留（grep 确认）
- [ ] Playwright 视觉回归测试通过（偏差 < 5%）
- [ ] 构建产物大小增加 < 50KB（gzipped）

---

## 6. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 手动转换 Tailwind 工作量大 | 工期延误 | 优先转换 P0/P1 组件，P2/P3 延后 |
| 样式不一致导致视觉混乱 | 用户体验下降 | 严格遵循 VIVY 设计令牌，不做创造性发挥 |
| 响应式布局未适配 | 移动端不可用 | 使用 CSS Grid/Flexbox，避免固定宽度 |

---

**文档版本：** v0.1  
**创建日期：** 2026-01-XX  
**维护者：** UI Migration Team  
**状态：** Draft - 待评审
