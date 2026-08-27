/**
 * VIVY-STUDIO-PLUGIN-HUB — Vivy Studio plugin marketplace.
 * Based on dsh-plugin-hub (DSH Plugin Hub contributors, MIT).
 * GitHub: https://github.com/dshplugin/dsh-plugin-hub
 *
 * Section header: brand title row (logo + title + version + controls) and the
 * tagline. No ad banner, no self-update badge: this build is a sealed first-
 * party marketplace with no update/upgrade paths.
 */
import { createElement as h, Fragment } from 'react'
import styles from '../../styles/Header.module.css'
import type { Translate } from '../../types.ts'
import { GITHUB_URL, PLUGIN_VERSION } from '../../logic/constants.ts'
import { GitHubIcon, LogoIcon } from '../ui/icons.tsx'

export function CatalogHeader({ t, langPath, statsTotal, statsVerified, onToggleLang, onAboutClick }: {
  t: Translate
  langPath: string
  statsTotal: number
  statsVerified: number
  onToggleLang: () => void
  /** 点击「关于」：打开本地固定的项目说明弹窗（引用原作者 dsh-plugin-hub） */
  onAboutClick: () => void
}) {
  return h(Fragment, null,
    h('div', { className: styles.header },
      h('div', { className: styles.headerTitleRow },
        // 第一行：logo + 标题（纯文本，无外部官网跳转）+ 版本号 + 右侧控件组
        h('div', { className: styles.brandTitle },
          h(LogoIcon),
          h('h1', { className: styles.title }, t('title')),
        ),
        // 版本号：纯只读展示（本构建关闭自我更新，无版本检查入口）
        h('span', { className: styles.versionBtn }, `v${PLUGIN_VERSION}`),
        h('div', { className: styles.headerRight },
          // 语言切换：按钮文字始终显示「要切到的语言」，点击即切
          h('button', {
            className: styles.langBtn,
            type: 'button',
            onClick: onToggleLang,
            title: t('toggleLangHint'),
            'aria-label': t('toggleLangHint'),
          }, langPath === 'zh/' ? 'EN' : '中文'),
          // 「关于」：本地固定说明（项目定位 + 引用原作者 dsh-plugin-hub）
          h('button', {
            className: styles.aboutBtn,
            type: 'button',
            onClick: onAboutClick,
            title: t('aboutDesc'),
            'aria-label': t('aboutTitle'),
          }, t('followUs')),
          // GitHub 源码图标：指向原作者仓库
          h('a', {
            className: styles.githubLink,
            href: GITHUB_URL,
            target: '_blank',
            rel: 'noopener noreferrer',
            title: t('githubHint'),
            'aria-label': t('githubHint'),
          }, h(GitHubIcon)),
        ),
      ),
      // 第二行：副标题（纯文本）
      h('div', { className: styles.tagline }, t('tagline', { total: statsTotal, verified: statsVerified })),
    ),
  )
}