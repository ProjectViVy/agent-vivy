# 综合审查 — findings 全量清单

分级：🔴 blocker（0）/ 🟠 should-fix（17，处置见括号）/ ⚪ note（节选代表，全量见各 lane 原始报告）。处置：**已修** = 修复轮落地；**记板** = `docs/TODO.md` §0.1 新行；**不修** = 记录即可。

## L1 合同符合性（PASS）

- 🟠 F1 §8「错误分类」槽位无 SDK 落点且未登记 → **记板**（CH-R-1）
- 🟠 F2 §9.3「import picoclaw/.workspace 禁止」verify 未实现 → **已修**（bannedImportPrefixes + `.workspace` 子串规则 + bad-picoclaw-import 夹具）
- 🟠 F3 §10 per-seam inspect 列表缺失只在 C2 日志、未上板 → **记板**（CH-R-5）
- ⚪ F4 §20「pack 空列表仍为空身体」字面不可执行（pack 拒绝零 --with）；不变量实际由 zz_register=nil + go.mod 零 SDK 保持 → 不修
- ⚪ F5 §14.3 qq「频道文本」未交付（源码核实 SDK 解不出群地址，偏离已记录）→ 建议合同行回写，待 SDK 补齐
- ⚪ F6 §12 payload 草图与实现不一致 → 建议回写合同（identifiers-only payload 是更优设计）；CH-C1-N2 已附 review 结论
- ⚪ F7 C2 日志「已登记 §0.1」主张不实（该行不存在）→ **已修**（补 CH-R-4 备忘行，注明启动期已兜底）
- ⚪ F8 `channels:` 信封与 settings overlay 无用户面文档（config.example.yaml/README 无章节）→ 不修（记入 findings；后继文档切片）
- ⚪ F9 §13「后切只读 RPC」与 C5 的 update RPC：判符合（写 settings.yaml 文件、重启生效，正是 §13 真正禁止物的反面），附注记录
- ⚪ F10 = F7 的闭环面（partitionChannels 兜底存在）

## L2 内核正确性+安全（PASS）

- 🟠 1 deliverCompleted/OnRunEvent nil 通道 panic 形（装配序不可达、注释与码不符）→ **已修**（两处守卫 + TestDeliverCompletedDropsUnregisteredChannel）
- ⚪ 2 终态先于 targets 注册的理论竞态（三语句 vs 一次模型往返；只丢不炸；CH-C3-N1 已跟踪）→ 不修
- ⚪ 3 会话 ID NUL 别名合并（平台 ID 均不可含 NUL；64 位截断够用）→ 不修
- ⚪ 4 allow_from 全串精确匹配无绕过；方向恒 fail-closed（大小写/空白只造成误拒）→ 不修
- ⚪ 5 `*_env` 走查：嵌套/别名/合并键/RPC 注入全不可扩权；CH-C6-N2 的实态比板面略真（Windows 大小写不敏感）→ 维持 OPEN
- ⚪ 6 丢弃类早退全部 return nil + 结构化日志（仅标识符）；策略丢弃无 Journal 痕迹（产品留白）→ 不修
- ⚪ 7 恰一次投递成立；关停窗口丢失有界（CH-C3-N1）+ CH-C4-N1 runes 上限无人执行 → 维持 OPEN
- ⚪ 8 StartAll/StopAll/Inspect 干净；双 StartAll 会重复登记（app 只调一次）→ 不修
- ⚪ 9 overlay 合并序健全（幽灵名先丢、partitionChannels 兜底、opaque node 不被改）
- ⚪ 10 RPC 全错误路径 + 密钥零回流复核通过；channel/update 未知字段静默忽略（无密私字段可走私）→ 不修
- ⚪ 11 Provenance nil 路径与 C1 前逐字节等价；伪造属内核内部调用者问题（CH-C1-N4 已跟踪）
- ⚪ 12 双引擎 provenance 列等价；v14 冻结夹具真实驱动原地升级；DSN 门控缺口=CH-C1-N5
- ⚪ 13 四厂商日志风险缓解全部在位（telego 脱敏/botgo 静默+断言/discordgo 钉 LogLevel/feishu 无 token 日志）

## L3 适配器横切（PASS；一致性矩阵 20 行 × 5 插件全 ✓ 或已记录偏离）

- 🟠 F1 dingtalk 网络级静默断线失聪（SDK Start 在 conn 存活时立即返回；仅优雅断连帧触发重拨；回环测试未覆盖死链）→ 注释已纠正 + **记板**（CH-C6-N3）；行为修复留后继
- 🟠 F2 dingtalk/feishu restart-after-stop 锁存未复位（feishu 潜在 Start 挂起）→ **已修**（Start 复位 + TestStartAfterStopStartsFresh ×2）
- ⚪ F3 telegram 迟到回调栅栏为 join 型（Stop ctx 已取消时可有一发在途；Host 丢弃安全）→ 不修
- ⚪ F4 SDK logger 姿态不对称（qq 静默/dingtalk 全盲/feishu 默认/telego 脱敏/discordgo 钉死）→ 后继 ChannelEnv 日志面（CH-C6-N1）一并定
- ⚪ F5 qq sender 形 `qq:user_<openid>` 唯一偏离裸 `<platform>:<id>`（已文档化，运维须知）
- ⚪ F6 dingtalk webhooks / qq chats 运行时 map 无上限（小字符串、picoclaw 同形）→ 不修
- ⚪ F7 已核实无动作项：feishu encrypt_key WS 惰性、discord 不重投故无去重必要、telegram 4096 即 API 上限
- 五项 SDK 主张源码核实全 CONFIRMED（telego 脱敏 logger.go:98-103；dingtalk WithAutoReconnect option.go:15 + 重连循环 Background ctx；lark v3.11 Start 返回 vs v3.9.4 select{}；botgo identify 帧 INFO 级；discordgo Identify 仅 LogDebug）

## L4 SDK/pack（PASS）

- 🟠 1 Listen 封禁可被方法调用/ListenPacket/tls.Listen 绕过（实证探针）→ **已修**（任意接收者方法名封禁 + tls.Listen + bad-channel-listen2 夹具；保守方向假阳性已注释）
- 🟠 2 pack 静默丢弃插件非 agent-vivy replace/exclude → **已修**（显式报错 + 分块/单行/合法三形测试）
- 🟠 3 双独立 module pack 无测试；重复 --with 不去重 → **已修**（TestPackTwoStandaloneModules 真构建 + 按解析目录去重 + 测试）
- 🟠 4 C2 日志「已登记 §0.1」不实 → **已修**（CH-R-4）
- ⚪ 5 verify 规则↔夹具完备表：14 负夹具一一对应；约 18 条规则无夹具（多为 apiVersion/semver 等既有行）；§9.3 picoclaw 行曾未实现 → **已修**
- ⚪ 6 ABI 家族相干；C8+ 最可能破签名排序：ChannelEnv 方法增长 > MediaStore 缺 Get > WebhookHandler 两方法集 > 空接口 TaskLifecycle/PipeServer 恒真断言陷阱（已注释未设防）> InboundMessage 缺显示名（加法安全）
- ⚪ 7 capabilities.go 对插件断言 MediaStore 属类目混淆（无害，无人实现）→ 不修
- ⚪ 8 pack 解析边角（行尾注释/`require(`无空格/块注释）→ 下游响亮失败，不修
- ⚪ 9 channel.go 九处「C4 pins the ABI」措辞过时（五耳均纯文本未实现可选能力）→ 修饰性，留后继
- 失败模式表 13 行全记录（原报告）；活树字节不变有测试链（zz_register/go.mod/go.sum 三者）

## L5 UI（PASS；删除清单 8 项零悬挂）

- 🟠 1 inspect 失败仍渲染「这一代没有耳朵」空态（错误仅 12px 工具条文本）→ **已修**（错误面板 + loadFailed i18n）
- 🟠 2 向导「输入平台凭据」超承诺（无 token 输入口是设计使然）→ **已修**（改「准备凭据环境变量」+ 界面外设 env + 重启指引，zh/en 对齐）
- 🟠 3 教程体过期且仅 zh（描述已删的凭据表单）→ **已修**（步骤重写为现实流程）
- 🟠 4 discord guild_id 占位符含审计禁语「留空表示不限制」（元数据字段）→ **已修**（改「留空 = 处理全部服务器」）
- ⚪ 5 zh.ts sandbox 域名占位符同短语（非通道面、语义本就如此）→ 不修
- ⚪ 6 channel-schema 导出的多组函数仅测试消费（「C6/C7 复用」理由已失效）→ 清理候选
- ⚪ 7 死 i18n 键（channels.channels、diva.channels 块、假统计 1/3 就绪）→ 清理候选
- ⚪ 8 重挂载不自动 refetch（手刷按钮存在；刷新间无缓存）→ 记录即可
- ⚪ 9 测试缺口：fail-closed 文案的值未钉（仅钉键名）、无组件级测试、api.test 只断方法名 → 后继补
- 删除台账 8 项全部零悬挂引用；数据真相/密钥纪律/i18n 对齐/a11y 全过

## L6 文档看板（PASS；9 日志逐条核验表全过）

- 🟠 1 幽灵分支 `feat/channel-c7a`（12a2a70 实落 c6 线；CH-C7a 横幅/C7b 两日志/TODO §0.2.7 四处措辞失实）→ **已修**
- 🟠 2 `ChannelEnv.Settings()` 未回写 CHANNEL-PACK §9.3 与 PLUGIN-SPEC §4（照文档实现编译不过）→ **已修**（两处补第五方法）
- 🟠 3 `UI-CHANNELS-BE` 陈旧行仍 OPEN（CH-C5 已领取交付）→ **已修**（置 DONE）
- 🟠 4 V0 架构文档 line~300 漏改「CN-01..16」→ **已修**（CN-17）
- ⚪ 5/6/7/8/9/10：C3 测试名拼写、C5 文件计数口径、CH-C1-N3/N4 路由陈旧（已随修复轮更新 N3；N4 维持）、C7a SDK 论证树外不可验（已披露）→ 记录即可
- 左扫：9 commit 零 scratch/二进制/routeTree 混入；新 Go/UI 代码零 TODO/FIXME；go.mod/go.sum 全程字节不变

## TEST-3（审查期间新立）

- 🟠 `just ui-e2e` 2 用例失败（runtime 全流程、welcome-wizard）——基线 82ecf14 复跑同败 → **记板**（§0.1 TEST-3：e2e 基线腐烂，与通道 EPIC 无关）
