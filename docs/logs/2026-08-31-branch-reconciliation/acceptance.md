# 验收

1. 在仓库根目录执行 `git branch --no-merged main`，结果只包含旧的 `wip/pre-submodule-root-20260829`。
2. 查看 `main` 最近提交，应依次看到四个分支合并提交：`a4e0c1a`、`1360659`、`349b9ac`、`80b2bfb`。
3. 执行 `just ci` 应全绿；启动 split 开发对后端 `:8787/healthz` 和 Vite `:3015/` 发起请求应成功。
4. 现有根目录未提交文件仍保持在合并前状态，未被本次分支回收覆盖。
