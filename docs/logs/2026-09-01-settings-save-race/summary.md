# settings.Save 并发写损坏文档（Windows rename access denied）

## 现象

`just ui-e2e` 的 model-refresh 用例在「新增模型」后偶发红色错误条
`internal error`，且刚新增的模型从注册表行里消失。错误来自 RPC
`internalError()`（`internal/rpc/control.go`，-32603，detail 有意丢弃，
不含内部细节）。

## 根因

`internal/app/settings` 的文档读写没有同步：

1. `Save` 使用固定 `path+".tmp"` 临时文件 —— 两个并发 Save 交错写同一
   临时文件，rename 出去的可能是一份损坏文档，后续 `Load` 解析失败。
2. Windows 上 `os.Rename` 覆盖一个仍被并发读句柄打开的文件会失败
   `Access is denied`（并发 `Load` 的 `os.ReadFile` 窗口）。

两条都会让下一次 `Load` 失败，RPC 层只看到 internal error。

## 修复（本目录对应的独立提交）

`internal/app/settings/settings.go`：

- 新增包级 `fileMu sync.Mutex`；`Load` 的 ReadFile 与 `Save` 的
  写临时文件 + rename 都在锁内，消除读/写句柄交叠。
- `Save` 改用 `os.CreateTemp(dir, base+".*.tmp")`：每次调用独占临时文件，
  并发写不再共享暂存区；失败路径都清理临时文件。
- 语义不变：Save 仍是整文档替换；Load 仍返回完整文档。
  跨 handler 的 Load→modify→Save（读改写）仍是 last-writer-wins，
  已开 TODO（未来 `settings.Update(path, fn)` 一类接口再收口）。

## 明确不做

- 不做 RPC 层错误细节透传（保持 internalError 不泄细节的契约）。
- 不在本窗口改读改写语义（见 TODO §0.1 新行）。
