/**
 * VIVY-STUDIO-PLUGIN-HUB — Vivy Studio plugin marketplace.
 * Based on dsh-plugin-hub (DSH Plugin Hub contributors, MIT).
 *
 * 头部「关于」弹窗：本地固定文案（不依赖远程接口），介绍项目定位与原作者出处。
 */
import { createElement as h } from 'react'
import type { MouseEvent } from 'react'
import styles from '../../styles/Modal.module.css'
import type { LocaleId, Translate } from '../../types.ts'
import { GITHUB_URL } from '../../logic/constants.ts'
import { CloseIcon } from '../ui/icons.tsx'

export function AboutModal({ version, lang, t, onClose }: {
  version: string
  lang: LocaleId
  t: Translate
  onClose: () => void
}) {
  const zh = lang !== 'en'
  return h('div', {
    className: styles.overlay,
    onClick: (e: MouseEvent<HTMLDivElement>) => {
      if (e.target === e.currentTarget) onClose()
    },
  },
    h('div', { className: `${styles.modal} ${styles.aboutModal}`, role: 'dialog', 'aria-modal': 'true' },
      h('div', { className: styles.modalHead },
        h('div', { className: styles.modalTitle }, t('aboutTitle')),
        h('button', {
          className: styles.modalClose,
          'aria-label': t('confirmCancel'),
          onClick: () => onClose(),
        }, h(CloseIcon)),
      ),
      h('div', { className: styles.modalDesc }, t('aboutDesc')),
      h('div', { className: styles.aboutContent },
        zh
          ? h('div', null, [
            h('p', null, `VIVY-STUDIO-PLUGIN-HUB v${version} — Vivy Studio 插件中心。`),
            h('p', null, '在 Vivy Studio 中浏览并安装社区插件；GitHub 插件默认克隆到 Vivy 源码树的 studio/ 目录并注册为本地 bundle。'),
            h('p', { style: { opacity: '0.65', marginTop: '8px' } },
              '本构建基于开源项目 dsh-plugin-hub 魔改（原作者：DSH Plugin Hub contributors，MIT License）。'),
            h('p', null,
              h('a', {
                href: GITHUB_URL,
                target: '_blank',
                rel: 'noopener noreferrer',
              }, 'https://github.com/dshplugin/dsh-plugin-hub')),
          ])
          : h('div', null, [
            h('p', null, `VIVY-STUDIO-PLUGIN-HUB v${version} — the Vivy Studio plugin marketplace.`),
            h('p', null, 'Browse and install community plugins inside Vivy Studio; GitHub plugins are cloned into the Vivy source tree under studio/ and registered as local bundles.'),
            h('p', { style: { opacity: '0.65', marginTop: '8px' } },
              'This build is a fork of the open-source dsh-plugin-hub (original authors: DSH Plugin Hub contributors, MIT License).'),
            h('p', null,
              h('a', {
                href: GITHUB_URL,
                target: '_blank',
                rel: 'noopener noreferrer',
              }, 'https://github.com/dshplugin/dsh-plugin-hub')),
          ]),
      ),
      h('div', { className: styles.modalActions },
        h('button', { className: styles.modalInstall, onClick: onClose }, t('doneBtn')),
      ),
    ),
  )
}