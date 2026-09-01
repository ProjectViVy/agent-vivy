# Acceptance

人如何确认：

1. 打开 http://127.0.0.1:3015 → 设置 → Vivy 功能 → 打开生命周期：页面顶部出现
   「换代权威在 Vivy Studio；此处仅只读检查，不提供创建/评测/提升入口。」
2. Generations / Evals / Promotions 三个 tab 只能看：没有创建 Generation 表单、
   没有「拒绝」按钮、没有启动/记录评测表单、没有「确认提升」表单。
3. Species tab 的 inspect 信息（协议、Generation、Artifact SHA、Policy、Recipe、
   工具与 Grants）与改动前一致。
4. 生命周期操作改在 Vivy Studio 进行（pack → eval → release → install → rollback），
   日常 Vivy 不再是换代操作入口。
