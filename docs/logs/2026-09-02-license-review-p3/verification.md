# Verification — 参考项目许可审查

日期：2026-09-02。

```text
ls .workspace/
  → 八目录（agent-wiki-library caveman crush deepseek-harness eino headroom oh-dsh smoke），
    确认 claude-code / rig 本地副本不存在 → 审查改对上游取证

curl -sL -H "Accept: application/vnd.github.raw+json" \
  https://api.github.com/repos/anthropics/claude-code/contents/LICENSE.md
  → 全文一行："© Anthropic PBC. All rights reserved. Use is subject to
    Anthropic's Commercial Terms of Service."（P3-1 证据）

curl -sL https://api.github.com/repos/anthropics/claude-code → license: None

curl -sL https://raw.githubusercontent.com/0xPlaygrounds/rig/main/LICENSE
  → 标准 MIT 全文，版权行 "Copyright (c) 2024, Playgrounds Analytics Inc."
    （P3-2 证据；正文逐字 MIT 条款，无附加限制）

just ci   （与本日 SR-4 / P2-1 两片 docs 合并过门）
  → CI-EXIT:0
```

备注：raw.githubusercontent.com 的 `anthropics/claude-code/main/LICENSE.md` 直取返回空
（分支/路径探测差异），改走 GitHub contents API 取得同样内容；两个来源互相印证。
