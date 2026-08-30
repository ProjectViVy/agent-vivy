# UI 迁移 - 国际化迁移计划

## 概述

本文档规划如何将 Agent Diva 的完整国际化内容迁移到 VIVY 的简化 i18n 系统中。

### 当前状态对比

| 项目 | Agent Diva | VIVY | 差距 |
|------|-----------|------|------|
| 文件大小 | en.ts: 1532 行, zh.ts: ~1500 行 | i18n.ts: 326 行（双语） | VIVY 缺少大量翻译键 |
| 组织结构 | 按功能模块分组（app/chat/settings等） | 扁平化键名空间 | 需要重构 |
| 语言支持 | 中文 + 英文 | 中文 + 英文 | ✅ 一致 |
| 运行时 | vue-i18n | 自研 translate() 函数 | ⚠️ 功能差异 |
| 命名空间 | 深度嵌套对象 | 单层键名 | 需要适配 |

---

## 1. Agent Diva 翻译键清单

### 已识别的命名空间（从 en.ts 提取）

```typescript
{
  app: { ... },                    // 应用级消息（欢迎、错误、状态）
  appDialog: { ... },              // 对话框按钮和标题
  emotion: { ... },                // 情感状态
  chat: { ... },                   // 聊天界面（输入框、工具调用、推理）
  convSidebar: { ... },            // 会话侧边栏
  settings: { ... },               // 设置通用
  language: { ... },               // 语言设置
  theme: { ... },                  // 主题设置
  sandbox: { ... },                // 沙箱策略
  compaction: { ... },             // 上下文压缩
  approvalCenter: { ... },         // 审批中心
  askUserQuestion: { ... },        // 用户问答
  planExecution: { ... },          // 计划执行
  notebook: { ... },               // 笔记本视图
  personaMemory: { ... },          // 人格记忆
  evolution: { ... },              // 进化提案
  skills: { ... },                 // Skills 市场
  mcp: { ... },                    // MCP 服务器
  channels: { ... },               // 通道管理
  providers: { ... },              // Provider 配置
  console: { ... },                // 控制台视图
  cronTasks: { ... },              // Cron 任务
  gatewayControl: { ... },         // Gateway 控制面板
  audit: { ... },                  // 审计日志
  tokenStats: { ... },             // Token 统计
  mate: { ... },                   // Desktop Mate (排除)
  vrm: { ... },                    // VRM Avatar (排除)
  voice: { ... },                  // 语音交互 (排除)
}
```

**估算：** 约 400-500 个唯一翻译键（排除 PET 相关后约 350-400 个）

---

## 2. VIVY 现有翻译键清单

### 已有键名（en 部分）

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

**计数：** 约 100 个唯一键

---

## 3. 迁移策略

### 3.1 命名空间设计

**问题：** VIVY 使用扁平键名（如 `newSession`），而 Agent Diva 使用嵌套命名空间（如 `convSidebar.newSession`）。

**方案 A：保持 VIVY 的扁平风格**
```typescript
// 优点：简单直接
// 缺点：键名可能冲突，不易维护
messages = {
  "zh-CN": {
    newSession: "新会话",
    chatNewSession: "新建对话",  // 前缀区分
    settingsNewSession: "创建新会话",
  }
}
```

**方案 B：引入命名空间分隔符**
```typescript
// 优点：结构清晰，易维护
// 缺点：需要修改 translate() 函数
messages = {
  "zh-CN": {
    "convSidebar.newSession": "新建对话",
    "chat.placeholder": "输入消息...",
  }
}

// translate() 需要支持点号访问
translate(locale, "convSidebar.newSession")
```

**方案 C：混合命名空间（推荐）**
```typescript
// 顶层键保留 VIVY 现有风格
// 新功能使用命名空间前缀
messages = {
  "zh-CN": {
    // 现有键保持不变
    newSession: "新会话",
    sessions: "会话",
    
    // 新增迁移键使用前缀
    chatPlaceholder: "输入消息...",
    chatSend: "发送",
    settingsTitle: "设置",
  }
}
```

**决策：方案 C** - 最小化破坏性变更，同时保持可扩展性

---

### 3.2 键名映射表

#### 3.2.1 直接复用（无需迁移）

以下 Agent Diva 键在 VIVY 中已有等价物：

| Agent Diva | VIVY | 备注 |
|-----------|------|------|
| `convSidebar.newSession` | `newSession` | ✅ |
| `convSidebar.sessions` | `sessions` | ✅ |
| `convSidebar.rename` | `rename` | ✅ |
| `convSidebar.delete` | `delete` | ✅ |
| `settings.save` | `save` | ✅ |
| `settings.cancel` | `cancel` | ✅ |
| `app.errorPrefix` | ❌ 缺失 | 需添加 |
| `chat.send` | `send` | ✅ |
| `chat.planMode` | `planMode` | ✅ |

#### 3.2.2 需要新增的键

**聊天界面（chat namespace）**
```typescript
// 约 80 个新键
chatPlaceholder: "输入消息...（Enter 发送）",
start: "开始与 DIVA 聊天~",
toolRunning: "正在调用工具...",
toolCall: "正在调用工具：{name}",
cleanToolCall: "工具调用：{name}",
toolSuccess: "成功",
toolFailed: "失败",
viewDetails: "查看详情",
hideDetails: "隐藏详情",
inputArgs: "输入参数：",
execResult: "执行结果：",
artifactSize: "结果大小：",
artifactId: "工件 ID：",
readHint: "读取提示：",
copyArtifactReference: "复制工件引用",
compactionRunning: "正在压缩上下文…",
compactionCompleted: "上下文压缩完成。",
compactionFailed: "上下文压缩失败；保留原始上下文。",
thinking: "深度思考中...",
retrying: "无响应，重试中 ({attempt}/{max})",
stalled: "仍在等待提供商响应...",
thinkingProcess: "思考过程",
thoughtProcess: "思维链",
clearChat: "清空聊天",
historySessions: "历史会话",
noHistory: "暂无历史",
deleteSession: "删除",
confirmDeleteSession: "确定要删除此会话吗？此操作不可撤销。",
unknownTime: "未知时间",
stopGeneration: "停止生成",
emptyModels: "没有保存的模型\n请在设置中添加",
manageModels: "管理模型...",
rawMeta: "结构化元数据",
viewRawMeta: "查看原始元数据",
hideRawMeta: "隐藏原始元数据",
prefAutoReasoning: "自动展开推理",
prefAutoTool: "自动展开工具详情",
prefAutoRawMeta: "自动显示原始元数据",
copy: "复制",
copied: "已复制",
edit: "编辑",
regenerate: "重新生成",
rewind: "回滚到此",
fork: "从此处分叉",
saveMemory: "保存到记忆",
pending: "待处理",
autoSelect: "自动选择",
attachment: "附件",
voice: "语音",
agentMode: "代理模式",
askMode: "问答模式",
conciseMode: "简洁模式",
deepThinking: "深度思考",
thinkingLow: "低",
thinkingMedium: "中",
thinkingHigh: "高",
contextUsage: "上下文使用",
queue: "排队",
stop: "停止",
agentModeDesc: "直接执行任务",
planModeDesc: "先规划后执行",
askModeDesc: "只读分析模式",
permissionCautious: "谨慎",
permissionCautiousDesc: "所有操作需确认",
permissionSmart: "智能",
permissionSmartDesc: "自动批准低风险操作",
permissionTrusted: "信任",
permissionTrustedDesc: "仅高风险操作需确认",
readonly: "只读",
enterToSend: "Enter 发送，Shift+Enter 换行",
reasoningSteps: "{count} 步推理",
reasoningProcessing: "处理中...",
reasoningThought: "思考了 {time} 秒",
reasoningProcessed: "处理了 {time} 秒",
reasoningNoThinking: "无思考",
thinkingMode: "思考模式",
thinkingModeAuto: "自动",
thinkingModeOn: "开启",
thinkingModeOff: "关闭",
attachFile: "附加文件",
uploading: "上传中...",
removeAttachment: "移除附件",
filePlaceholder: "[文件]",
// ... 更多
```

**会话侧边栏（convSidebar namespace）**
```typescript
// 约 20 个新键
convSidebarTitle: "会话历史",
convSidebarSearch: "搜索会话...",
convSidebarPinned: "已固定",
convSidebarNoHistory: "暂无会话",
convSidebarRefresh: "刷新",
convSidebarNoResults: "无匹配结果",
convSidebarPin: "固定",
convSidebarUnpin: "取消固定",
convSidebarClose: "关闭侧边栏",
convSidebarOpen: "打开会话历史",
convSidebarStatusIdle: "空闲",
convSidebarStatusRunning: "运行中",
convSidebarStatusCompleted: "已完成",
convSidebarStatusError: "错误",
convSidebarUntitled: "未命名",
convSidebarJustNow: "刚刚",
convSidebarMinutesAgo: "{count} 分钟前",
convSidebarHoursAgo: "{count} 小时前",
convSidebarDaysAgo: "{count} 天前",
convSidebarShowAll: "显示全部",
convSidebarShowPinned: "仅显示已固定",
```

**设置面板（settings namespace）**
```typescript
// 约 60 个新键
settingsGeneral: "通用",
settingsMcp: "MCP",
settingsSkills: "技能",
settingsProviders: "提供商",
settingsChannels: "通道",
settingsNetwork: "网络",
settingsLanguage: "语言",
settingsMate: "伙伴",  // ⚠️ PET 相关，排除
settingsAbout: "关于",
settingsSelfEvolution: "自我进化",
settingsSaved: "已保存",
settingsAdd: "添加",
settingsEdit: "编辑",
settingsName: "名称",
settingsModel: "模型",
settingsApiKey: "API 密钥",
settingsApiBase: "API 地址",
settingsProvider: "提供商",
settingsActions: "操作",
settingsSandbox: "沙箱",
settingsSandboxDesc: "隔离执行环境与安全策略",
// Provider 管理
providersList: "已配置的提供商",
providersAdd: "添加提供商",
providersEdit: "编辑提供商",
providersDelete: "删除提供商",
providersTest: "测试连接",
providersTestSuccess: "连接成功",
providersTestFailed: "连接失败",
providersCustom: "自定义提供商",
providersBuiltIn: "内置提供商",
// Channel 管理
channelsList: "已配置的通道",
channelsAdd: "添加通道",
channelsEdit: "编辑通道",
channelsDelete: "删除通道",
channelsTest: "测试连接",
channelsEnable: "启用",
channelsDisable: "禁用",
// Skills 市场
skillsInstalled: "已安装技能",
skillsMarketplace: "技能市场",
skillsInstall: "安装",
skillsUninstall: "卸载",
skillsUpdate: "更新",
skillsSearch: "搜索技能",
skillsNoInstalled: "暂无已安装技能",
skillsNoMarketplace: "市场加载失败",
// MCP 服务器
mcpServers: "MCP 服务器",
mcpAddServer: "添加服务器",
mcpEditServer: "编辑服务器",
mcpDeleteServer: "删除服务器",
mcpConnected: "已连接",
mcpDisconnected: "未连接",
mcpConnecting: "连接中",
mcpTools: "可用工具",
mcpResources: "可用资源",
// 审计日志
auditLog: "审计日志",
auditGuiLog: "GUI 操作日志",
auditRawLog: "原始事件流",
auditStructuredEvents: "结构化事件",
auditSearch: "搜索日志",
auditFilter: "筛选",
auditExport: "导出",
auditNoLogs: "暂无日志",
// Token 统计
tokenStats: "Token 统计",
tokenUsed: "已使用",
tokenLimit: "限制",
tokenRemaining: "剩余",
tokenResetTime: "重置时间",
tokenWarning: "警告阈值",
tokenCritical: "临界阈值",
```

**审批中心（approvalCenter namespace）**
```typescript
// 约 30 个新键
approvalCenterTitle: "待审批事项",
approvalPending: "待处理",
approvalApproved: "已批准",
approvalDenied: "已拒绝",
approvalCancelled: "已取消",
approvalExpired: "已过期",
approvalApprove: "批准",
approvalDeny: "拒绝",
approvalCancel: "取消",
approvalGrantOnce: "一次性授权",
approvalGrantAlways: "始终授权",
approvalToolName: "工具名称",
approvalArguments: "参数",
approvalRiskContext: "风险上下文",
approvalDiff: "差异对比",
approvalViewDiff: "查看差异",
approvalEditAtSource: "在源位置编辑",
approvalStale: "审批已过期，请刷新",
approvalOutcomeUnknown: "审批结果未知",
approvalRequestFailed: "请求失败",
approvalPageLimit: "已达到页面加载限制",
approvalNoPending: "暂无待审批事项",
approvalLoading: "加载中...",
approvalError: "加载失败：{error}",
approvalRefresh: "刷新",
approvalFilterAll: "全部",
approvalFilterPending: "待处理",
approvalFilterDecided: "已决策",
approvalSortByExpiry: "按过期时间排序",
approvalSortByCreated: "按创建时间排序",
```

**计划执行（planExecution namespace）**
```typescript
// 约 40 个新键（⚠️ 依赖后端 Plan 域模型）
planTitle: "计划",
planGoal: "目标",
planPhase: "阶段",
planStatus: "状态",
planRevision: "版本",
planMarkdown: "计划文档",
planSummary: "摘要",
planStrategy: "策略",
planSteps: "步骤",
planTodos: "待办事项",
planCreatedAt: "创建于",
planUpdatedAt: "更新于",
planAwaitingApproval: "等待批准",
planExecuting: "执行中",
planVerifying: "验证中",
planCompleted: "已完成",
planFailed: "失败",
planPartial: "部分完成",
planApprove: "批准并执行",
planReject: "拒绝",
planReturnToDraft: "退回草稿",
planCancel: "取消执行",
planValidationError: "验证错误",
planValidationWarning: "验证警告",
planTodoPending: "待处理",
planTodoInProgress: "进行中",
planTodoCompleted: "已完成",
planTodoCancelled: "已取消",
planTodoBlocks: "阻塞",
planTodoBlockedBy: "被阻塞",
planMaterializeTodos: "生成待办事项",
planContextRetain: "保留上下文",
planContextCompact: "压缩上下文",
planContextClear: "清除上下文",
planExecutionStarted: "计划执行已开始",
planExecutionFailed: "计划执行失败：{error}",
planExecutionCompleted: "计划执行完成",
```

**记忆与 Persona（memory/persona namespace）**
```typescript
// 约 50 个新键
memoryTitle: "记忆管理",
memoryShortTerm: "短期记忆",
memoryLongTerm: "长期记忆",
memoryCore: "核心记忆",
memorySearch: "搜索记忆",
memoryNoResults: "未找到记忆",
memoryDelete: "删除记忆",
memoryEdit: "编辑记忆",
memoryCreatedAt: "创建于",
memoryUpdatedAt: "更新于",
memoryConfidence: "置信度",
memorySource: "来源",
personaTitle: "人格工作区",
personaFrozenCore: "冻结核心",
personaEditable: "可编辑部分",
personaSave: "保存人格",
personaRevert: "还原",
personaVersionHistory: "版本历史",
personaCurrentVersion: "当前版本",
personaCreatedBy: "创建者",
personaLastModified: "最后修改",
evolutionTitle: "进化提案",
evolutionProposals: "待审查提案",
evolutionNoProposals: "暂无待审查提案",
evolutionApply: "应用",
evolutionReject: "拒绝",
evolutionPreview: "预览变更",
evolutionDiff: "差异对比",
evolutionRiskAnalysis: "风险分析",
evolutionApplied: "提案已应用",
evolutionRejected: "提案已拒绝",
notebookTitle: "笔记本",
notebookSearch: "跨会话搜索",
notebookNoResults: "未找到相关内容",
notebookHits: "命中 {count} 条",
notebookViewSource: "查看来源",
```

**控制台与诊断（console namespace）**
```typescript
// 约 30 个新键
consoleTitle: "控制台",
consoleGatewayStatus: "Gateway 状态",
consoleHealthy: "健康",
consoleUnhealthy: "异常",
consoleChannels: "通道连接",
consoleChannelConnected: "已连接",
consoleChannelDisconnected: "未连接",
consoleCronTasks: "定时任务",
consoleNoTasks: "暂无定时任务",
consoleLogs: "日志",
consoleLogLevel: "日志级别",
consoleLogInfo: "信息",
consoleLogDebug: "调试",
consoleLogError: "错误",
consoleLogSearch: "搜索日志",
consoleLogStreaming: "实时流",
consoleLogPaused: "已暂停",
consoleTokenStats: "Token 统计",
consoleTokenUsage: "使用情况",
consoleTokenBudget: "预算",
consoleTokenWarning: "警告",
consoleConfigEditor: "配置编辑器",
consoleConfigReadOnly: "只读模式",
consoleCannotEdit: "生产环境不可编辑",
```

**欢迎向导（onboarding namespace）**
```typescript
// 约 20 个新键
onboardingWelcome: "欢迎使用 Vivy",
onboardingGetStarted: "开始使用",
onboardingSkip: "跳过",
onboardingNext: "下一步",
onboardingBack: "上一步",
onboardingFinish: "完成",
onboardingStep1Title: "配置 LLM 提供商",
onboardingStep1Desc: "选择一个 AI 模型提供商并输入 API 密钥",
onboardingStep2Title: "配置搜索服务",
onboardingStep2Desc: "Bocha Search 用于增强联网搜索能力",
onboardingStep3Title: "选择导航目标",
onboardingStep3Desc: "你想从哪里开始？",
onboardingNavigateChat: "开始聊天",
onboardingNavigateProviders: "管理提供商",
onboardingNavigateNetwork: "网络设置",
onboardingNavigateConsole: "控制台",
onboardingApiKeyHint: "API 密钥仅存储在内存中，重启后需重新输入",
onboardingTestConnection: "测试连接",
onboardingConnectionSuccess: "连接成功",
onboardingConnectionFailed: "连接失败：{error}",
```

---

## 4. 实施步骤

### Step 1：扩展 i18n.ts 结构（1 天）

**任务：**
1. 保持现有的 `messages` 对象结构
2. 添加新的翻译键到 `zh-CN` 和 `en` 两个语言包
3. 确保类型安全（更新 `TranslationKey` 类型）

**示例代码：**
```typescript
const messages = {
  "zh-CN": {
    // === 现有键保持不变 ===
    appName: "Vivy",
    newSession: "新会话",
    // ... 其他现有键
    
    // === 新增：聊天界面 ===
    chatPlaceholder: "输入消息...（Enter 发送）",
    chatStart: "开始与 Vivy 聊天~",
    chatToolRunning: "正在调用工具...",
    // ... 更多 chat 键
    
    // === 新增：会话侧边栏 ===
    convSidebarTitle: "会话历史",
    convSidebarSearch: "搜索会话...",
    // ... 更多 convSidebar 键
    
    // === 新增：设置面板 ===
    settingsGeneral: "通用",
    settingsMcp: "MCP",
    // ... 更多 settings 键
    
    // === 新增：审批中心 ===
    approvalCenterTitle: "待审批事项",
    approvalPending: "待处理",
    // ... 更多 approval 键
    
    // === 新增：计划执行 ===
    planTitle: "计划",
    planGoal: "目标",
    // ... 更多 plan 键
    
    // === 新增：记忆与 Persona ===
    memoryTitle: "记忆管理",
    personaTitle: "人格工作区",
    // ... 更多 memory/persona 键
    
    // === 新增：控制台 ===
    consoleTitle: "控制台",
    consoleGatewayStatus: "Gateway 状态",
    // ... 更多 console 键
    
    // === 新增：欢迎向导 ===
    onboardingWelcome: "欢迎使用 Vivy",
    onboardingGetStarted: "开始使用",
    // ... 更多 onboarding 键
  },
  en: {
    // 对应的英文翻译
    // ...
  },
} as const;
```

### Step 2：更新 translate() 函数以支持插值（半天）

**当前实现：**
```typescript
export function translate(locale: Locale, key: TranslationKey, values?: Record<string, string | number>): string {
  let text: string = messages[locale][key] ?? messages.en[key];
  for (const [name, value] of Object.entries(values ?? {})) text = text.replace(`{${name}}`, String(value));
  return text;
}
```

**问题：** 如果键不存在于指定语言，会 fallback 到英文，但如果英文也不存在，会返回 `undefined` 导致运行时错误。

**改进：**
```typescript
export function translate(locale: Locale, key: TranslationKey, values?: Record<string, string | number>): string {
  const localeMessages = messages[locale];
  const fallbackMessages = messages.en;
  
  let text: string | undefined = localeMessages[key];
  if (text === undefined) {
    text = fallbackMessages[key];
    if (text === undefined) {
      console.warn(`Missing translation for key: ${key}`);
      return key;  // 返回键名作为占位符
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

### Step 3：编写翻译完整性检查脚本（半天）

**目的：** 确保 `zh-CN` 和 `en` 的键完全一致，避免遗漏。

**脚本示例：**
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

### Step 4：人工审核翻译质量（1-2 天）

**检查项：**
1. **语气一致性**：Agent Diva 使用亲昵语气（"Master~ 💕"），VIVY 应保持专业简洁
2. **术语统一**：确保相同概念在不同地方使用相同译名
3. **长度适配**：英文翻译不应过长，避免 UI 溢出
4. **文化适配**：某些表达可能需要本地化调整

**示例调整：**
```typescript
// Agent Diva（过于亲昵）
welcome: 'Hello, Master~ 💕\n\nI am DIVA. I will always be with you...'

// VIVY（专业简洁）
welcome: '你好，我是 Vivy。请问有什么可以帮你？'
```

### Step 5：添加翻译键文档（半天）

**目的：** 为后续开发者提供翻译键的使用指南。

**文档结构：**
```markdown
# 翻译键使用指南

## 命名规范
- 使用小驼峰命名：`chatPlaceholder` 而非 `chat_placeholder`
- 按功能模块分组前缀：`chat*`, `settings*`, `approval*`
- 避免重复：优先复用现有键

## 插值语法
使用 `{variableName}` 占位符：
```typescript
translate(locale, "chatRetrying", { attempt: "1", max: "3" })
// "无响应，重试中 (1/3)"
```

## 添加新键
1. 在 `zh-CN` 和 `en` 中同时添加
2. 运行完整性检查脚本
3. 更新本文档的键列表
```

---

## 5. 工作量估算

| 任务 | 工时 | 负责人 |
|------|------|--------|
| Step 1: 扩展 i18n.ts 结构 | 1 天 | Frontend |
| Step 2: 更新 translate() 函数 | 0.5 天 | Frontend |
| Step 3: 编写完整性检查脚本 | 0.5 天 | Frontend |
| Step 4: 人工审核翻译质量 | 1-2 天 | Frontend + PM |
| Step 5: 添加翻译键文档 | 0.5 天 | Frontend |
| **总计** | **3.5-4.5 天** | |

---

## 6. 验收标准

- [ ] `zh-CN` 和 `en` 的键数量完全一致
- [ ] 所有翻译键都有非空字符串值
- [ ] 完整性检查脚本通过
- [ ] 随机抽样 20 个键进行人工审核，质量合格
- [ ] 翻译键文档已更新
- [ ] UI 中使用新翻译键的地方正常显示（无 `undefined`）

---

## 7. 后续优化建议

1. **自动化翻译同步**：使用工具（如 i18next-parser）自动提取代码中的翻译键
2. **翻译管理平台**：考虑使用 Lokalise/Crowdin 等专业平台协作翻译
3. **多语言支持**：未来可添加日语、韩语等亚洲语言
4. **动态加载**：对于大型应用，可按需加载语言包以减少初始体积

---

**文档版本：** v0.1  
**创建日期：** 2026-01-XX  
**维护者：** UI Migration Team  
**状态：** Draft - 待评审
