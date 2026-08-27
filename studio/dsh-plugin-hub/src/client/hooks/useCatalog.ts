/**
 * DSH Plugin Hub — the community plugin marketplace for DeepSeek Harness.
 * Website: https://dsh-plugin.org
 * GitHub: https://github.com/dshplugin/dsh-plugin-hub
 *
 * Catalog data + view state for the Plugin Hub section.
 *
 * Owns the online-data pipeline (live fetch from api.dsh-plugin.org), the local
 * installed-plugin table, and the filter/search/sort/install-status view
 * state, exposing the derived visible list and per-category counts.
 */
import { useEffect, useMemo, useRef, useState } from 'react'
import type { HubPlugin, LocaleId } from '../types.ts'
import type { SortKey } from '../logic/constants.ts'
import { fetchCatalog, fetchStats } from '../data/catalog.ts'
import { fetchInstalled } from '../data/host.ts'
import {
  installedItemsOf, installedNameOf, installedVersionOf,
} from '../logic/installed.ts'
import type { InstalledItem, InstalledVersionSignal } from '../logic/installed.ts'

/** 插件市场自身仓库：VIVY-STUDIO-PLUGIN-HUB 不显示在目录里（自己不进自己的插件列表） */
const SELF_REPO = 'dshplugin/dsh-plugin-hub'

/** 市场各排序的默认方向：全部按倒序（Star/Fork 多、更新/收录近的在前） */
const SORT_DEFAULT_DIR: Record<SortKey, 'asc' | 'desc'> = {
  sortStars: 'desc', sortForks: 'desc', sortUpdated: 'desc', sortNewest: 'desc',
}

export function useCatalog(lang: LocaleId) {
  /** 目录插件（仅保留人工验证通过的条目） */
  const [plugins, setPlugins] = useState<HubPlugin[] | null>(null)
  /** 收录/精选统计（官网 /api/stats.json 实时拉取） */
  const [stats, setStats] = useState<{ total: number; verified: number } | null>(null)
  const [failed, setFailed] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [category, setCategory] = useState('all')
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<SortKey>('sortStars')
  /** 当前排序方向：点同一个排序按钮切换 正序/倒序；点新排序用该排序的默认方向（与已安装视图一致） */
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  /** 目录加载「超时自动强制刷新」重试计数：宿主重启/代理未就绪时请求可能长时间挂起，
   *  界面停在「正在加载插件数据…」假死 —— 超时自动重拉，最多自动重试 2 次后转失败态；
   *  用户手动刷新（reload）时清零重新计数。 */
  const loadRetriesRef = useRef(0)
  /** 目录加载超时阈值：超过即判定挂起，强制刷新一次（服务端代理 curl 上限 20s，客户端 12s 先兜底） */
  const LOAD_TIMEOUT_MS = 12000
  /** 挂起后最多自动重试次数：避免代理持续不通时无限循环刷新（每次重试仍 12s，2 次后转失败界面可手动重试） */
  const MAX_LOAD_RETRIES = 2

  /** 点击排序按钮：同一按钮切换正/倒序，新按钮用默认方向 */
  function toggleSort(key: SortKey) {
    if (key === sort) setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    else { setSort(key); setSortDir(SORT_DEFAULT_DIR[key]) }
  }

  /** 当前 profile 已安装插件：npm 包名 -> manifest spec（来自宿主本地路由） */
  const [installed, setInstalled] = useState<Record<string, string>>({})
  /** 安装时记录的目录信号：repo(小写) -> { version, updatedAt }（来自宿主本地路由）；
   *  npmPackage 为 npm 优先通道反查命中的包名映射（目录数据未下发时客户端靠它把依赖 key 匹配回仓库） */
  const [versions, setVersions] = useState<Record<string, InstalledVersionSignal>>({})
  /** 每个依赖在系统上的安装目录（profile/node_modules/<包名>），详情视图展示用 */
  const [installPaths, setInstallPaths] = useState<Record<string, string> | null>(null)
  /** 已加载进运行中 loader 的包名（官方 ctx.loader 对账）：装完未重启的新插件不在其中 */
  const [loadedNames, setLoadedNames] = useState<string[] | null>(null)
  /** 真正的 dsh 插件包名（包内声明 dsh 配置 / 在 profile bundles 清单）：非 dsh 插件不提示「待重启」 */
  const [dshCapableNames, setDshCapableNames] = useState<string[] | null>(null)

  // 拉取目录 + 统计。在线 API 已只返回 verified；再过滤一次，保证只展示人工验证通过的插件。
  useEffect(() => {
    let cancelled = false
    // 本次加载是否已收尾（成功或失败）：超时定时器只对「还在加载中」生效
    let settled = false
    // 中止控制器：请求挂起时（宿主重启/代理抖动）超时自动重试会换新请求，
    // 旧请求在此中止释放连接，避免挂起堆积
    const controller = new AbortController()
    setPlugins(null)
    setStats(null)
    setFailed(false)
    const load = () => Promise.all([fetchCatalog(lang, controller.signal), fetchStats(controller.signal)])
    load()
      .then(([list, stats]) => {
        if (cancelled) return
        settled = true
        // 插件市场不显示自己：VIVY-STUDIO-PLUGIN-HUB 从目录里排除自身条目，
        // 防止「自己出现在自己的插件列表里、还能自己安装自己」；统计计数同步减 1 保持与列表一致。
        // 目录数据已排除自身，此处再兜底过滤一次，防旧快照仍含自身条目。
        const hadSelf = list.some((p) => p.source?.repo === SELF_REPO)
        setPlugins(list.filter((p) => p.compatibility?.status === 'verified' && p.source?.repo !== SELF_REPO))
        if (stats) {
          setStats(hadSelf
            ? { total: stats.total - 1, verified: stats.verified - 1 }
            : { total: stats.total, verified: stats.verified })
        }
      })
      .catch(() => {
        if (cancelled) return
        settled = true
        setFailed(true)
      })
    // 加载超时兜底：宿主刚重启/代理未就绪时目录请求可能长时间挂起，界面停在「正在加载插件数据…」
    // 像假死。超过阈值未收尾 → 强制刷新列表（重拉一次，最多 MAX_LOAD_RETRIES 次自动重试），
    // 重试仍失败 → 转失败界面（显示「重试」按钮，手动刷新时计数清零重新开始）。
    const timer = window.setTimeout(() => {
      if (cancelled || settled) return
      if (loadRetriesRef.current < MAX_LOAD_RETRIES) {
        loadRetriesRef.current += 1
        setReloadKey((k) => k + 1)
      } else {
        setFailed(true)
      }
    }, LOAD_TIMEOUT_MS)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
      controller.abort()
    }
    // lang 必须进依赖：切换界面语言要重新拉对应语言的数据文件，
    // 否则英文模式仍停留在 plugins.zh.json，描述/名称全是中文
  }, [reloadKey, lang])

  /** 刷新当前 profile 已安装插件表；宿主未挂本地路由时静默降级为空表。 */
  const refreshInstalled = async () => {
    const data = await fetchInstalled()
    if (data === null) return
    setInstalled(data.installed)
    setVersions(data.versions)
    setInstallPaths(data.paths)
    setLoadedNames(data.loaded)
    setDshCapableNames(data.dshCapable)
  }

  // 首次进入拉取已安装表（依赖宿主 webServer 服务）
  useEffect(() => { refreshInstalled() }, [])

  /** 插件是否已安装：匹配 Git spec 中的 owner/repo，或 npm 通道安装的依赖包名；命中返回 npm 包名。 */
  const installedName = (p: HubPlugin): string | null => installedNameOf(p, installed, versions)

  /** 该插件安装时记录的目录版本（无记录/未安装 → null）。 */
  const installedVersion = (p: HubPlugin): string | null => installedVersionOf(p, versions)

  /** 已安装项统一列表（目录元数据 + 运行时信息合并）：驱动「已安装」tab。 */
  const installedItems = useMemo<InstalledItem[]>(
    () => installedItemsOf(plugins, installed, versions, installPaths, loadedNames, dshCapableNames),
    [plugins, installed, versions, installPaths, loadedNames, dshCapableNames],
  )

  /** 当前分类下的插件（「全部」时为整个目录）。 */
  const categoryPlugins = useMemo(() => {
    if (!plugins) return []
    if (category === 'all') return plugins
    return plugins.filter((p) => p.category === category)
  }, [plugins, category])

  const visible = useMemo(() => {
    if (!plugins) return []
    const q = query.trim().toLowerCase()
    const list = plugins.filter((p) => {
      if (category !== 'all' && p.category !== category) return false
      if (!q) return true
      return (
        (p.displayName ?? '').toLowerCase().includes(q) ||
        (p.description ?? '').toLowerCase().includes(q) ||
        (p.topics ?? []).some((topic) => topic.toLowerCase().includes(q))
      )
    })
    const dir = sortDir === 'asc' ? 1 : -1
    return [...list].sort((a, b) => {
      // 比较器统一用「升序写法」再乘 dir：desc 时 dir=-1 翻成降序（Star/Fork 最多、更新/收录最近在前）
      if (sort === 'sortStars') return ((a.stats?.stargazers_count ?? 0) - (b.stats?.stargazers_count ?? 0)) * dir
      if (sort === 'sortForks') return ((a.stats?.forks_count ?? 0) - (b.stats?.forks_count ?? 0)) * dir
      if (sort === 'sortNewest') return (a.dates?.addedAt ?? '').localeCompare(b.dates?.addedAt ?? '') * dir
      return (a.dates?.repoUpdatedAt ?? '').localeCompare(b.dates?.repoUpdatedAt ?? '') * dir
    })
  }, [plugins, category, query, sort, sortDir, installed])

  /** Per-category plugin counts shown on the category chips. */
  const categoryCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const p of plugins ?? []) {
      if (p.category) counts[p.category] = (counts[p.category] ?? 0) + 1
    }
    return counts
  }, [plugins])

  return {
    plugins,
    stats,
    failed,
    reload: () => { loadRetriesRef.current = 0; setReloadKey((k) => k + 1) },
    installed,
    installedItems,
    installedName,
    installedVersion,
    refreshInstalled,
    category,
    setCategory,
    query,
    setQuery,
    sort,
    sortDir,
    toggleSort,
    visible,
    total: plugins?.length ?? 0,
    categoryCounts,
  }
}
