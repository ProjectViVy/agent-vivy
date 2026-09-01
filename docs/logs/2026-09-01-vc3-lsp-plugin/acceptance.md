# Acceptance — VC-3 切片 1（人工可判）

## 怎么判断它成功了

1. **默认 EXE 没有 LSP（D4 口径）**
   - 主线 `just run` / 日常 `vivy.exe` 的模型工具表里没有 `lsp_diagnostics`
     （committed Register 只装第一方 channel 插件）。
2. **打包后模型侧多一个只读诊断工具**
   - `vivy-sdk pack --with lsp` 产出新 EXE + `dist/<gen>/generation.json`，
     其中 `recipe.plugins` 含 `lsp`、`tools` 含 `{"name":"lsp_diagnostics",
     "readonly":true}`。用 `vivy-sdk inspect-artifact dist/<gen>` 核对。
3. **真实会话里（装了语言服务器后）**
   - 装好 gopls（或 typescript-language-server / pyright-langserver /
     rust-analyzer）后，在 pack 出的 EXE 会话里让模型调用
     `lsp_diagnostics {"path":"main.go"}`：
     - 有类型/lint 错误 → 输出 `main.go:13:2: error: undefined: x [compiler]`
       形态的行；
     - 文件干净 → 输出 `no diagnostics`；
     - 服务器未安装 → 调用失败并给出 `lsp: start gopls: ...` 的可读错误
       （不静默）。
   - 本机无 gopls，开发者冒烟步骤：
     1. `go install golang.org/x/tools/gopls@latest`
     2. `vivy-sdk pack --with lsp`，运行新 EXE，会话中调用上述工具。
4. **安全边界（代码审阅可判）**
   - 插件源码里 `os/exec` 仍被 verifier 拒封（spawn 只能走 `Env.Spawn`）；
   - 子进程 cwd 固定在 run workspace，命令仅允许 PATH 裸名或 workspace
     相对路径（绝对路径/`..` 逃逸 → ErrInvalidArgs）；
   - 无 `proc.spawn` grant 的插件调 Spawn → ErrDenied；
   - tool seam manifest 声明 proc.spawn → `vivy-sdk verify` 失败
     （fixture `sdk/internal/testdata/bad-procspawn-seam/`）。

## 回滚

分支 `feat/vc1a-bash-tool` 上 revert 本切片 commit 即可：全部改动
（kernel 能力 + 插件 + fixture + 日志）自包含，无迁移、无配置项。
