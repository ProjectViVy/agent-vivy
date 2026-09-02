# Acceptance — P2-1 Diva 能力清单

人如何确认生效：

1. 打开 `docs/research/diva-capability-inventory.md`：10 个域表 + §11 统计 +
   §12 再入规则；每行应有 Diva 证据路径与 Vivy 现状两列。
2. 抽查可证伪行：
   - 行 2.1 应记 47 个 provider 预设（可在 `agent-diva-providers/src/providers.yaml`
     数 `- name:` 验证）；
   - 行 9.1 应记 176+1 个 Tauri 命令（grep `#[tauri::command]` 可验证）；
   - 行 1.10（消息编辑/回退/分叉）应指向 UI-CHAT-ACT 在办状态。
3. `docs/TODO.md` §0.1：P2-1 行 DONE 2026-09-02 + §10 有对应记录；
   `docs/research/OPEN-ITEMS.md` P2-1 同步。
4. 用途判据：下一个想做 Defer 能力的人，应能在清单里找到该能力的 Diva 对照、
   现状差距与"必须先写提案"的提示，而无需重新考古 agent-diva。
