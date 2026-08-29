# verification — remove subscriptions plugin

## Commands run

```text
git -C studio rm -r dsh-plugin-subscriptions        # tree removed, staged
git -C studio commit -m "chore(plugins): drop dsh-plugin-subscriptions snapshot"
git -C studio status --short                        # clean
git -C studio log --oneline -3                      # 5b6cb4e on top of b7de607

Remove-Item -Recurse -Force data/studio-home/plugins/subscriptions
Test-Path data/studio-home/plugins/subscriptions    # False

# live Studio (127.0.0.1:3090), read-only probes:
GET /                                 -> title "Vivy Studio";
                                       boot entries contain no "subscription",
                                       no "@studio" anywhere in boot JSON
GET /dsh-plugin-hub/installed         -> installed/versions/paths/loaded/dshCapable
                                       contain none of dsh-plugin-subscriptions

rg -n "dsh-plugin-subscriptions" studio (excluding node_modules)  # 0 matches
```

## `just ci` gate

`just ci` 在本交付物完成时运行，**失败于 `fmt-check`**，全部是根树既有脏区
（另一并行 lane 的半成品 compaction 代码）：

```text
internal\app\compaction.go
internal\rpc\control.go
internal\storage\sqlite\compaction.go
internal\storage\postgres\compaction.go
internal\runtime\compaction_service.go
internal\runtime\compaction_policy.go
internal\runtime\compaction_middleware.go
```

这些文件（`internal/app`、`internal/rpc`、`internal/storage/*`、
`internal/runtime/compaction_*`）与本交付物零交集——本交付只动
`studio/` 子模块（独立 git 仓）与 `data/studio-home/`（gitignore 的 Studio scratch）。
按规则将失败原因记录于此：非本变更引入，属根树既有未提交 lane 状态；
该 lane 收口后 `just ci` 才可全绿。Go/UI 源码本交付未触碰任何文件。

## Studio 侧验证结论

- 子模块删除提交后 `git -C studio status` 干净。
- profile 层（package.json bundles / deps、`vivy-source-plugins.json`、
  `gro.ngilp-hsd-versions.json`、node_modules、cordis）经复核均不含该插件。
- 运行中应用的装载清单与 Hub 的 installed 接口均无该插件；无需重启服务器。