# VC-3 记录侧：file_versions 落表 + filetracker stale-read + 插件写挂链（内核）

## What changed

按 2026-09-01 O1..O6 拍板（MVP 先对齐 Crush，恢复侧暂缓 RB-L2-DEFER），落地文件版本链的**记录侧**内核部分：

1. **存储层（sqlite + postgres 双后端）**
   - 新表 `file_versions`（session_id, run_id, path, version, content_hash, content, created_at；UNIQUE(session_id, path, version)；每 (session,path) 保 20 版=O2；单版 >1MB 跳过不截断）与 `file_reads`（stale-read 标记，PRIMARY KEY(session_id,path)）。
   - sqlite migration019；postgres base schema 追加 + schemaV18Upgrade + schemaVersion=18。
   - 新契约 `storage.FileVersionStore`：`RecordFileMutation`（单事务完成 Crush 式挂链：首次见档存基线 → 链上 latest ≠ 磁盘旧内容先插外部中间态 → 追加新内容，hash 去重；超限跳过）+ `TrackFileAccess`（upsert 标记）+ `LastFileAccess`。**刻意无版本读 API**——恢复消费方（RPC/UI）在 RB-L2-DEFER，不留无消费方的接口面。
   - 新表无外键，会话删除走显式 DELETE（规避 compactions 外键暴露的删除顺序隐患）。
   - conformance 套件新增 CN-18（接口契约：记录不报错、tracker 往返、会话删除级联）；链语义细节（基线/中间态/去重/保留/超限）由 sqlite+postgres 各自的包内测试直查表断言。

2. **写路径挂链（internal/runtime）**
   - 新 seam `tools.FileVersionRecorder`（与 WriteDiagnosticsSource 同款可选注入模式）+ `runtime.FileVersionRecorder` 适配器（best-effort，失败只记 Warn 日志）。
   - `EinoFilesystemBackend` 挂三点：`ReadFile` 成功后 TrackAccess；`WriteFile` 成功后 RecordMutation + TrackAccess；`WriteFile` 写前 stale-read 守卫（filetracker：磁盘 mtime 晚于会话最后访问标记 → 拒绝"changed on disk after the last read; read it again before editing"；**从未 tracked 的路径放行**——守卫针对 stale 编辑，不是强制 read-before-write）。
   - patch/multiedit/eino 原生 write_file/edit_file 全部经由 `WriteFile`，一处挂链全覆盖。
   - app 装配：`fileBackend.SetFileVersionRecorder(runtime.NewFileVersionRecorder(backend, nil))`。

3. **插件写路径挂链（internal/pluginhost，切片2）**
   - `pluginhost.Adapt` 增加 `tools.FileVersionRecorder` 参数（nil 保持原行为），hostedEnv 的 `OpenWrite` 由裸 `os.OpenFile(O_TRUNC)` 换成内核侧捕获包装器：**打开时不截断**，旧内容先快照且在 Close 前一直保留在磁盘上；插件写先缓冲（上限 1MB），Close 才是破坏性时刻——truncate + flush + RecordMutation + TrackAccess。
   - 超限语义：新内容或旧内容无法完整快照（>1MB / 读取失败）→ 跳过链记录（绝不记截断快照毒化链），但 access 标记照常刷新；被放弃未 Close 的 writer 不产生任何记录。
   - 关闭幂等（最多记录一次）；无 session 上下文、nil recorder 均静默直通。sdk/plugin 面零改动，插件源码零改动（lsp_rename 自动入链）。

4. **顺带修复（同一删除级联关注面）**
   - `DeleteSession`（sqlite+postgres）级联清单补 `session_compactions`——postgres 侧原存在隐性 bug：compactions 行声明了 `FOREIGN KEY REFERENCES sessions(id)` 而删除清单漏了该表，删除带 compaction 的会话会 FK 报错。与新两表级联同属"会话删除覆盖全部子表"关注面，一并修复。

## What was explicitly NOT done

- **恢复侧全部暂缓**（files/versions RPC、files/restore、会话级回退 UI）→ RB-L2-DEFER，MVP 验证后再议。
- **download/bash 写文件不进版本链**：均为绕过文件工具的带外变更（O6 已知边界）；bash/download 改动会刷新磁盘 mtime，stale-read 守卫会按设计要求模型先读再改。
- 无 Journal 新事件、无 RPC、无 UI 变更（拍板范围外）。
