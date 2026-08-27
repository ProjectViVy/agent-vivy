/**
 * DSH Plugin Hub — the community plugin marketplace for DeepSeek Harness.
 * Website: https://dsh-plugin.org
 * GitHub: https://github.com/dshplugin/dsh-plugin-hub
 *
 * Constants: site URLs, category metadata and sort keys. Pure data — no
 * side effects, no network — so every consumer shares one definition.
 */
import type { LocaleId } from '../types.ts'

/** 插件中心官网地址。 */
export const SITE_URL = 'https://dsh-plugin.org/'

/** 插件中心源码仓库：头部右上角 GitHub 图标的跳转地址（指向原作者的 dsh-plugin-hub） */
export const GITHUB_URL = 'https://github.com/dshplugin/dsh-plugin-hub'

/** Hub 自身在目录数据里的仓库标识：目录加载时按它把自己排除在插件列表之外。 */
export const HUB_REPO = 'dshplugin/dsh-plugin-hub'

/** 构建时由 tsdown 从 package.json 注入的插件版本号（见 tsdown.config.ts define）。 */
declare const __PLUGIN_VERSION__: string

/**
 * 插件当前版本号，供头部标题展示「DSH Plugin Hub v0.1.1」。
 * tsdown 构建时用 define 把 __PLUGIN_VERSION__ 替换成 package.json 的版本号；
 * node --test 直接 import 本模块时该标识符不存在，typeof 守卫兜底为空串。
 */
export const PLUGIN_VERSION: string =
  typeof __PLUGIN_VERSION__ === 'string' ? __PLUGIN_VERSION__ : ''

export const CATEGORY_ORDER = [
  'interface', 'session', 'memory', 'tools', 'agent', 'workflow',
  'integration', 'model', 'dev', 'knowledge', 'fun',
]

/** 分类标签按界面语言取词；未知 key 原样返回。 */
export function categoryLabel(
  map: Record<string, { zh: string; en: string }>,
  key: string,
  lang: LocaleId,
): string {
  return map[key]?.[lang] ?? key
}

export const CATEGORY_LABELS: Record<string, { zh: string; en: string }> = {
  interface: { zh: '界面与体验', en: 'UI & Experience' },
  session: { zh: '会话与消息', en: 'Sessions & Messages' },
  memory: { zh: '记忆与上下文', en: 'Memory & Context' },
  tools: { zh: '工具与能力', en: 'Tools & Capabilities' },
  agent: { zh: '技能与智能体', en: 'Skills & Agents' },
  workflow: { zh: '工作流与自动化', en: 'Workflow & Automation' },
  integration: { zh: '集成与连接', en: 'Integrations & Connections' },
  model: { zh: '模型与推理', en: 'Models & Reasoning' },
  dev: { zh: '开发与运维', en: 'Development & Operations' },
  knowledge: { zh: '数据与知识', en: 'Data & Knowledge' },
  fun: { zh: '娱乐', en: 'Entertainment' },
}

/** 分类 Tab 的紧凑标签（英文模式空间受限时使用）。 */
export const CATEGORY_SHORT_LABELS: Record<string, { zh: string; en: string }> = {
  interface: { zh: '界面体验', en: 'UI Exp' },
  session: { zh: '会话消息', en: 'Sessions' },
  memory: { zh: '记忆上下文', en: 'Memory' },
  tools: { zh: '工具能力', en: 'Tools' },
  agent: { zh: '技能智能体', en: 'Agents' },
  workflow: { zh: '工作流', en: 'Workflow' },
  integration: { zh: '集成连接', en: 'Integrations' },
  model: { zh: '模型推理', en: 'Models' },
  dev: { zh: '开发运维', en: 'Dev' },
  knowledge: { zh: '数据知识', en: 'Knowledge' },
  fun: { zh: '娱乐', en: 'Fun' },
}

export const SORTS = ['sortStars', 'sortForks', 'sortUpdated', 'sortNewest'] as const
export type SortKey = (typeof SORTS)[number]
