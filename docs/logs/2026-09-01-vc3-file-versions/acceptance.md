# Acceptance（人如何确认）

本切片为内核记录侧，无独立 UI 面。可观察行为在 agent 会话的文件工具链路里：

1. **版本链记录**：让 agent 在会话里 write/patch/multiedit 某文件后，`data/vivy.db` 的 `file_versions` 表出现对应行——首次写某路径产生两行（改动前基线 + 改后内容），后续编辑各追加一行；同内容重复写不追加；单文件超过 20 版后只保留最新 20 版。`file_reads` 表随 read_file 出现 (session, path, read_at) 行。
2. **stale-read 守卫**：会话中 agent read 过某文件后，若外部（人手改、bash、download）改动了它，agent 再 patch 会被拒绝，工具结果含 "changed on disk after the last read; read it again before editing"；agent 重新 read 后即可正常编辑。
3. **插件写同样入链**：agent 在会话里调用 lsp_rename 等插件写工具改动文件后，`file_versions` 出现与内核写工具同款的链行（基线 + 改后）；插件写取消/中途失败（未 Close）时文件保持旧内容不变、无链行。
4. **会话删除级联**：删除一个改过文件的会话，`file_versions` / `file_reads` / `session_compactions` 中该会话的行一并消失（postgres 下此前会因 compactions 外键报错，现已修复）。
5. **无感面**：未装配 workspace 的部署、无 session 上下文的调用行为与升级前一致；失败的历史写入只产生 Warn 日志，绝不阻塞文件工具本身（best-effort 承诺）。
