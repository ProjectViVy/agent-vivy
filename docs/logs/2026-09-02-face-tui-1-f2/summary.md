# FACE-TUI-1 F2 — seam-face SDK 契约 + 内核 FaceHost RunFace + 出厂 faces/headless 器官 + pack `--face`

日期：2026-09-02。范围：FACE-TUI-1 的 F2 切片（VIVY-FACE-PACK.md PR 3 合同的落地）。F1（2026-09-02-face-tui-1-f1）与 §14 四问拍板（2026-09-02-face-pack-14-rulings）是其前置，本切片不改变两者结论。

## 交付内容

**SDK 契约（sdk/plugin）**
- `SeamFace Seam = "face"`：face 器官是内核 FaceHost 托管的"嘴"——是控制面客户端，永远不是模型工具（§6）。
- face 族 grants：`tty`（终端读写）、`argv`（命令行参数）、`rpc.client`（经 FaceEnv.Call 调控制面）。
- `face.go`：`Face`（Kind/Run）、`FaceEnv`（Call/OnEvent）、`FaceOptions`（Prompt/ContinueNewest/Out/Err——launcher 持有 stdout/stderr，器官不得自开）、`FaceResult{Status}`、`FaceConstructor`。

**SDK 校验（sdk/internal）**
- manifest：`face` 信封（kind/listen）+ `checkFaceManifest`：零 tools、kind ∈ web|tui|headless、`listen` 必须 false（本批，监听是 face 自己的 effect）、grants 限 face 族；非 face seam 声明 face 对象报错。
- inspect：构造器规则按 seam 分派——face 器官要求 `func New(plugin.FaceOptions) plugin.Face`（`hasNewFace`），其余 seam 仍要求 `func New() plugin.Plugin`。
- verify：seam 传入 checkSources。

**内核（internal）**
- `internal/generated/face/zz_face.go`：默认注册器返回 nil——committed body 无 face 器官，`vivy run` 走既有内核 headless 循环。
- `internal/app/facehost.go`：`RunFace(ctx, cfg, ctor, opts)`——gateway-less 组合（WithoutEars+WithoutGateway）+ DialControl(net.Pipe) + `faceEnv` 适配器（Call→Peer.Call；OnEvent 注册通知回调；服务器带 ID 请求回 null）。器官只见 FaceEnv，不见 App。
- `cmd/vivy/run.go`：`face.Register()` 非 nil 时走 `RunFace`（器官侧），否则既有 `RunHeadless` 原样不动；退出码映射 completed=0/cancelled=2/其余=1 两条路径一致。
- `domain.AssemblyRecipe` 加 `Face` 字段。

**faces/headless 器官（独立 module）**
- `faces/headless/`：自有 go.mod（`module example.com/vivy/faces/headless`，`replace agent-vivy => ../..`），零第三方依赖。manifest：seam face、grants tty/argv/rpc.client、`face{kind headless, listen false}`。
- `headless.go` 只说控制面 JSON-RPC：initialize → 会话解析（--continue 取 session/list 最新；否则 session/create、标题=prompt 截 60 rune）→ turn/start（face=headless）→ run/subscribe（after_seq 0，回放补齐订阅前事件）→ 事件渲染（model.delta→Out、model.completed 补行、tool.started/failed→Err、run.failed/cancelled/completed 终点）。**§14④**：tool.approval_required / user.question_required → 响亮通知 + run/cancel（durable cancelled 终点），不等待、不 yolo、不绕 HITL。语义逐条对照内核 headless.go。

**pack（sdk/internal/pack.go）**
- `--face <organ>`：至多一个（"一代一张嘴"，§14③）；resolveFaceDir（faces/ 惯例候选）+ verify（seam 必须 face）+ build-time overlay `internal/generated/face/zz_face.go`（生成 `Register() plugin.FaceConstructor { return <pkg>.New }`）。
- faces module 是 standalone module → 复用既有 -modfile 合并路径（pack.mod/pack.sum 临时对，live go.mod/go.sum 不动）。
- Artifact 增 `face` 记录（name/version/kind/grants/source_ref/tree_hash）；`recipe.face` 入账。

**CI**
- justfile：fmt-check 加 `faces`；plugin-ci 扩展为遍历 `plugins/` + `faces/` 两个 module 根。

## 明确未做（留给 F3 与后续切片）

- F3：`faces/tui` 交互式 TUI 器官（bubbletea 等）——独立切片。
- faces/web：web face 仍是网关内嵌 UI 的形态；本切片未迁移。
- `face.listen: true`、webhook/listen channel 类 face 传输：本批明确拒绝。
- face grants 的运行时细分仲裁：本批由 verifier 把关词汇，FaceEnv 单一实现。

## 复核要点

- committed body（不带 --face 打包）行为零变化：默认注册器 nil → RunHeadless 原路径；internal/generated/face/zz_face.go 是手工维护默认，pack 只在 build overlay 替换、从不写工作树。
- 租户 Journal 零接触：冒烟在 %TEMP% 独立目录进行（见 verification.md）。
