# Vivy Channel Pack — Super-Channel Contract

> **2026-09-09 v1 specification coverage:** The super-channel, ChannelHost, envelope, ledger, and
> transport semantics in this document remain valid; all descriptions of `seam: channel`, `vivy.plugin/v0`,
> `vivy.generation/v0`, `[]plugin.Plugin`, the old ABI, and compatibility migration are historical
> records and must not be used as the basis for new implementations. The only v1 mechanism is
> `std/channel@v1` + ChannelHost + `vivy.module/v1` + Generation Recipe, and it retains no v0 API. The
> canonical specification is `VIVY-MODULE-STANDARD.md`, `VIVY-PORT-CATALOG.md`,
> `VIVY-PLUGIN-SPEC.md`, and `VIVY-ASSEMBLY.md`.
>
> Status: **product semantics adopted; the plugin assembly mechanism is superseded by the v1 specification**.
> Follows `SELF-EVOLVING-GATEWAY.md`, `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, **`VIVY-STUDIO.md`**, PRD §5.0 / D-016.
> Date: 2026-08-30
>
> Comparative evidence (read-only, not dependencies):
> - The channel system in `.workspace/picoclaw` (Go adapter samples)
> - agent-diva 2026-08-18–08-23: channel retirement, capability envelope, A2A research, and NeuroLink reservation
> - `github.com/cloudwego/eino-ext/a2a` (A2A protocol/codec reference; the example server is not the product path)
> - DeepSeek Harness's "capability seam / registration as effect"
> - This repository's ADR-015 pack overlay (`internal/generated/plugins/zz_register.go`)

Related:

- `VIVY-ASSEMBLY.md` — named by what they are; this batch's channel adapters use `plugins/` + `seam: channel` and do not add a `channels:` recipe key
- `VIVY-PLUGIN-SPEC.md` — user-layer `plugins/<name>/`; this document extends it with `seam: channel`
- `SELF-EVOLVING-GATEWAY.md` — installing a plugin = building a new version; default `Register()` is empty
- `VIVY-GATEWAY-AND-STUDIO.md` — NG-10 model-visible ≡ accounted; NG-11 rejects a second body
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — remote-control existing turns; not an ear
- `VIVY-FACE-PACK.md` — local mouth (web / tui / headless); channel must not replace the face
- `VIVY-CHANNEL-EVOLUTION.md` — this EPIC's evolution architecture tree; child AGENT PLANs are in `docs/plans/channel-epic/`
- `.workspace/picoclaw/pkg/channels/README.zh.md` — division of responsibility between protocol adapters and Manager

---

## 0. One Sentence

> **The super-channel is the world ingress in the kernel, not a collection of five bots.**
> Host, the standard envelope, and the optional capability matrix always live in the species.
> Telegram / Discord / Feishu / DingTalk / QQ are all `seam: channel` plugins.
> True removal = delete the recipe line and `pack` again. Deactivation = yaml. The default body has no ears.

This translates DSH's "registration as effect" into Vivy's cold plug/unplug model: borrow the capability seam and inspectable registration, not hot mounting, not treating the Journal as a plugin, and not an external `telegram.exe`.

---

## 1. Decision Record

| Date | Decision |
|---|---|
| 2026-08-25 | Write the built-in `channels/` cold plug/unplug proposal. C0 not adopted. |
| 2026-08-27 | Port Diva's seven-platform configuration UI to the frontend (localStorage only). `UI-CHANNELS-BE`. |
| 2026-08-30 | **Adopt the super-channel.** Host + envelope + capability matrix are the contract spine. A2A / NeuroLink come later, using the same envelope. |
| 2026-08-30 | **Pluginize all five in this batch.** Put the five adapters in `plugins/<name>/` and use the established `Register()` overlay. Do not add a `channels/` directory or a new `RegisterChannels()`. |
| 2026-08-30 | **Eino-native A2A = borrow the protocol, not the example server.** Later, `plugins/a2a` uses the `models`/`transport` from `eino-ext/a2a`; do not use `RegisterServerHandlers(adk.Agent)` as the gateway. The loop remains `Service.Run`. |

The five names in this batch are: `telegram`, `discord`, `feishu`, `dingtalk`, `qq`.

Intentional exception: `VIVY-ASSEMBLY.md` says "first-party code must not go in `plugins/`." The channel adapters in this batch are **not kernel organs**; they are optional ears, so they go in `plugins/` and are tagged by seam. If first-party organs become more numerous later, the directory may move to `channels/<name>/`; the **ABI remains unchanged**.

---

## 2. Three Doors (Do Not Merge into One Seam)

```text
The world speaks first             The mouth humans see             Remote control of existing turns
───────────────                    ──────────                      ──────────
ChannelHost / WorldPort            FaceHost                          ACP (separate proposal)
  plugins/telegram …               web / tui / headless              observe, cancel, approve
  later: neurolink                 one main face per generation       not a new ear
  later: a2a
```

- **Face** is the mouth. A channel must not replace the local UI.
- **ACP** is the control plane. It redirects to an existing Run and does not invent a second inbound world.
- **ChannelHost** is the ear. Chat platforms, NeuroLink, and A2A all enter through this door.

A2A is not a second loop. Remote tasks map to an existing Vivy `Run`. NeuroLink is not another Telegram-style configuration card; it is a later heavyweight plugin, and the Listen surface is owned by Host.

---

## 3. The Feeling to Solve

Allow pure Go implementations of Telegram / Feishu adapters. Do not let authors feel they are modifying the gateway, and do not let editing a yaml file grow new ears on the body.

The criteria for "feels like building a channel plugin":

1. The working directory contains only `plugins/<name>/`. Do not open `internal/` for daily work.
2. The world enters only through the channel contract in `sdk/plugin`. Host / Journal / `Service.Run` are kernel components invisible to the adapter.
3. Identity is the name in the manifest and `seam: channel`, not a `.go` file referenced by `engine.go`.
4. To add or remove a channel, the author changes the **recipe**, not a blank-import list. `pack` generates `Register()`.
5. Before it runs, it is only source. Once it runs, it is part of a generation's EXE body. The registry committed into the species remains empty (ADR-015).

picoclaw uses `init()` + Gateway blank imports to weld twenty-one protocols into one process. Those are hot-tree entries that cannot be cleanly unloaded. Vivy's composition happens in `vivy-sdk pack`, whose output is a whole EXE generation.

---

## 4. Four Layers

```text
L0  Kernel Host (never pluginized)
    admission · session mapping · accounting · dispatch Run · outbound · capability discovery
    optional listen surface (not :8787) · later child-process supervision

L1  Standard envelope (the real contract of the super-channel)
    identity / thread / typed parts / streaming / delivery / reserved task
    Every world ingress outside Face is normalized here

L2  Adapter ABI (sdk/plugin, seam: channel)
    Required: Start / Stop / Send / PublishInbound
    Optional: Host type-assertion discovery; the adapter does not hold Service

L3  Plugin organs plugins/<name>/
    five in this batch + later neurolink / a2a
    Recipe `plugins:` names what is compiled into this generation; default Register() = nil
```

| Layer | Where it lives | What it feels like to change | How it appears in the live body |
|---|---|---|---|
| Kernel Host | `internal/` (package name set during implementation) | Modifying Vivy | Always compiled in |
| Channel plugin | `plugins/<name>/`, `seam: channel` | Building Telegram / Feishu | Recipe `plugins:` + pack |
| Configuration envelope | `channels:` in `config.yaml` | Adjusting knobs | May name only **names listed by inspect** |

Today `pluginhost` adapts everything into `tools.Tool`. A channel **must not** use this path. `Seam()` becomes the routing criterion for the first time: tools enter the tool table, and channels enter Host.

---

## 5. What to Borrow from DSH / picoclaw / Diva, and What to Refuse

### 5.1 Borrow

| Source | Idea | Vivy form |
|---|---|---|
| DSH | Name things by what they are | inspect tag is channel (from seam), not tool |
| DSH | Capability seam = Definition + Provider + Consumer | Manifest + adapter + **consumed only by ChannelHost** |
| DSH | Registration is effect | grant, child-process handle, and webhook path enter inspect; removal is defined |
| DSH | Model-visible ≡ accounted | Inbound traffic must have `channel.inbound`; it cannot masquerade as a local UI `user` row |
| DSH | Two event surfaces | typing / placeholder use the live surface; provenance uses the Journal |
| DSH | Killable world | Later: same-binary `vivy channel --name telegram` (not a second body) |
| picoclaw | `Channel` + optional capability interfaces | Start/Stop/Send; Host discovers typing / editor / media / listen |
| picoclaw | Structured Inbound / Outbound / MediaPart | Copy the fields, not Manager into the kernel package |
| picoclaw | Session dimensions `chat` / `topic` / `sender` | Map to an existing Vivy Session; do not create another JSONL system |
| picoclaw | Shared webhook mux | **Owned by Host**; the adapter is only a Handler |
| ADR-015 | Live registry is empty; only pack overlay links entries in | The same `internal/generated/plugins/zz_register.go` |
| Diva 2026-08-23 | Channel envelope must be reusable by A2A | L1 reserves `task_id` / parts / stream; private metadata cannot be the main contract |
| Diva 2026-08-23 | A2A is a northbound adapter | Later `plugins/a2a`; Task = existing Run |
| Diva 2026-08-23 | NeuroLink is a heavyweight reservation | Later `plugins/neurolink`; not a Telegram-style ready/missing_fields card |
| Eino ADK | `ChatModelAgent` + `Runner` (already pinned by Vivy) | Species loop. ChannelHost and later A2A both feed `Service.Run`; do not start another Runner |
| Eino ADK | `AgentAsTool` / DeepAgent | In-process multi-agent behavior, not the A2A protocol. Not touched in this batch |
| eino-ext/a2a | Agent Card, JSON-RPC, Task, parts, Stream | Later `plugins/a2a` codec borrows `models` + `transport` |
| Diva 2026-08-18 | Unverified channel retirements | slack / whatsapp / matrix / irc / mattermost / nextcloud are not in this batch |

### 5.2 Refuse

| Idea | Reason |
|---|---|
| Forcibly putting Telegram into the existing `tool` / `tool-world` seam | Those are the model's hands. A channel is the world speaking first |
| Putting the five adapters in `internal/` | Pluginize all five in this batch; the kernel keeps only Host |
| Adding `channels/` + `RegisterChannels()` in this batch | A parallel second overlay. Reuse the established `Register()` ABI |
| Making Discord appear by editing `config.yaml` | Configuration is not a plugin system. A name absent from the body causes startup failure |
| Growing `TelegramSettings` in the kernel `Config` | Writing non-core attributes back into the core makes the kernel grow for every protocol |
| `.dll` / Go `plugin` / external `telegram.exe` | NG-11; one-click Windows; secret boundary |
| Letting the plugin call `net.Listen` itself | A plugin is not a process; Host opens the Listen surface |
| Making Host / Journal / Session mapping a plugin | The environment cannot be a population member; that would be a new species, not a new generation |
| Compiling 21 protocols into the default body | Size, crash domain, and the personal gateway in PRD §5.0.1 |
| WeChat QR, native WhatsApp, and Delta Chat external RPC | Unofficial clients, TTY, and a second body |
| Letting Telegram users directly call `approval/respond` | A new HITL principal; belongs to the ACP proposal, not the first cut |
| Importing `github.com/sipeed/picoclaw` | import-lint forbids `.workspace`; the dependency tree would explode. MIT permits rewriting the adapters |
| Allowing an empty `allow_from` | Diva GUI contract; personal gateway must fail-closed |
| Regressing DingTalk to a webhook text bot | Diva / picoclaw already use Stream; Octos webhook is not the baseline |
| Discord `voice.go` / `pion/webrtc` | Explicitly out of scope for this batch |
| Making the web UI `seam: channel` | Face is another door |
| Using `eino-ext/a2a.RegisterServerHandlers(adk.Agent)` as the Vivy A2A gateway | A second loop: self-built Hertz, default in-memory TaskStore, direct `Runner.Run/Resume`, bypassing Journal / Policy / approvals / sandbox; Listen is not owned by Host. Borrow the protocol, not this server wiring |

---

## 6. Three Layers of Cold Plug/Unplug (Do Not Collapse into One Switch)

```text
1. Generation replacement (true cold)
   delete telegram from the recipe → pack → the next-generation EXE has neither this body part nor telego

2. Runtime deactivation (the card remains in its slot)
   config enabled: false → Host does not Start / calls Stop
   Code remains in the body; inspect still lists it as compiled-in

3. Instance kill (the part of DSH unload that can be honestly realized in Go)
   Kill the `vivy channel --name telegram` child process
   Classify the failure as channel_lost; the species and Journal remain alive
   The first cut may be in-process; the contract must already be written along process boundaries
```

"Clean removal" applies only to (1). See §13 for the criteria.

`enabled: false` is not unloading. A live EXE **does not** dynamically load any channel code—the same as Kind B.

---

## 7. ChannelHost (Kernel, Never Pluginized)

Host is on the "kernel never pluginized" list alongside the Journal writer, Policy, Secret resolver, inspect, and `vivy worker` supervision.

Exclusive responsibilities that adapters must not perform:

1. **Admission.** Empty `allow_from` = reject `Start` (fail-closed). Forbid picoclaw / Diva GUI's "an empty list means speaking to the whole world." `"*"` is not allowed in the first cut.
2. **Session mapping.** `(channel, chat_id[, topic_id])` → existing or new Vivy `Session`. Local UI Session and channel Session do not merge by default.
3. **Accounting.** First `channel.inbound`, then `Message(role=user)` with provenance. Call `Service.Run`.
4. **Outbound.** Send terminal state (and future incremental output, if implemented) back through the adapter's `Send`. Live-surface typing / placeholder does not enter the Journal.
5. **Secrets.** Resolve only `token_env` in the envelope (or the symmetric `*_env`). Values are never written to configuration or the Journal.
6. **Capability discovery.** Host uses type assertions on adapters; degrade when an optional interface is absent, and do not require all five packages to implement the full set.
7. **Supervision.** Supervise later child-process lifecycles; a crash = `channel_lost`, not species death.
8. **Listen surface (if any; not in the five plugins of this batch).** A separate listen, loopback by default; **do not** attach it to `:8787` `/rpc`. The adapter declares only a path + `http.Handler`. NeuroLink uses this surface.

Host **does not** know `parse_mode`, Feishu encryption algorithms, or Telegram forum markdown. Those belong to the plugin's `settings`.

---

## 8. Standard Envelope (L1, Fixed in the First Cut)

Reserve the fields now; add behavior later. Do not use `Metadata map[string]string` as the main contract. picoclaw has already promoted Peer / MessageID to first-class fields; inherit them directly.

The five v1 plugins may fill only text; Host must already understand these slots:

| Slot | Five v1 chat plugins | Future NeuroLink | Future A2A |
|---|---|---|---|
| `channel` + `chat_id` + `sender` | Required | pipe's chat/sender | `channel=a2a` + contextId |
| `message_id` / `reply_to` / `topic_id` | Fill when present | Fill when present | A2A message ID |
| `parts[]` (text / media-ref / structured) | Text only for now | Text + media later | Text + artifact |
| Live surface: typing / delta / placeholder / finalize | Connect when capable | delta/reply are protocol core | StreamResponse |
| `run_id` / `task_id` | Host writes; adapter only echoes back | Same | A2A taskId = Vivy run_id |
| delivery / edit / delete | Optional interface | Optional | Task state |

Capability matrix (Host discovers; inspect lists compiled-in vs advertised vs enabled):

```text
Required     Start Stop Send
Interaction  Typing  Edit  Delete  Reaction  Placeholder  Stream
Media        MediaSender
Ingress      WebhookHandler / ListenHandler   ← Host owns Listen
Reliability  HealthChecker  error classification (rate-limit / temporary)
Heavyweight  TaskLifecycle (A2A)  PipeServer (NeuroLink)
```

The first cut for the five packages: required capabilities plus optional capabilities already stable in that platform's picoclaw implementation and not blocking the text loop. Stream / media / groups / approval cards all come later, but **Host must already understand these interfaces**. Otherwise this is not a super-channel, only five bots.

---

## 9. Channel Plugins (L3)

### 9.1 Directory (This Batch)

```text
plugins/
  hello-fs/                 # existing; seam: tool-world
  telegram/                 # this batch; independent go.mod
    vivy-plugin.json
    plugin.go               # New() plugin.Plugin, and implements Channel
    settings.go             # types and decoding for non-core fields
    plugin_test.go
    README.md
    go.mod                  # depends only on sdk/plugin + the platform SDK
  discord/
  feishu/
  dingtalk/
  qq/

internal/generated/plugins/zz_register.go
  // Always empty in the version committed into the species (ADR-015)
  func Register() []plugin.Plugin { return nil }
```

An independent Go module is a **hard requirement for this batch**: the import graphs for default `go build ./cmd/vivy` and `just ci`'s `go test ./...` must not reach `github.com/mymmrac/telego` and similar packages. Only the `Register()` generated by the `pack` overlay imports `plugins/telegram`. `hello-fs` may remain in the species module because it has no large SDK.

As soon as someone casually imports telegram in `internal/`, cold plug/unplug is invalidated.

### 9.2 Manifest `vivy-plugin.json`

```json
{
  "apiVersion": "vivy.plugin/v0",
  "name": "telegram",
  "version": "0.1.0",
  "seam": "channel",
  "module": ".",
  "grants": ["channel.poll", "secret.read"],
  "channel": {
    "transport": "poll",
    "max_message_runes": 4096
  }
}
```

| Field | Rule |
|---|---|
| `name` | Matches the directory name; unique within this generation's recipe |
| `seam` | Must be `channel`. Do not write `tool` / `tool-world` |
| `grants` | Upper bound for this package, frozen after pack into the generation. Runtime cannot widen it through configuration |
| `channel.transport` | Only `poll` is allowed in this batch (outbound long polling or outbound WS client). `webhook` / `listen` are later and require the corresponding grant |
| `tools` | **Forbidden**. A channel is not a model tool |

`vivy-sdk verify` applies channel rules to `seam: channel` (zero tools, must implement Channel), while `tool` / `tool-world` still require at least one tool.

### 9.3 Code Contract (Written Along Process Boundaries)

Authors import only `agent-vivy/sdk/plugin`. The public surface expands alongside the existing `SeamTool`; it does not create another SDK:

```go
const SeamChannel Seam = "channel"

const (
    GrantChannelPoll    Grant = "channel.poll"     // outbound long polling / WS client
    GrantChannelWebhook Grant = "channel.webhook"  // declare path only; Listen belongs to Host
    GrantChannelListen  Grant = "channel.listen"   // NeuroLink, etc.; Listen remains Host-owned
    GrantChannelA2A     Grant = "channel.a2a"      // later
    GrantSecretRead     Grant = "secret.read"      // read-only env_key value
)

type Channel interface {
    Name() string
    Seam() Seam // must be SeamChannel
    Grants() []Grant
    Start(ctx context.Context, env ChannelEnv) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutboundMessage) (ids []string, err error)
}

type ChannelEnv interface {
    Secret(envKey string) (string, error) // fail-closed; value never enters logs
    HTTP() *http.Client                   // outbound; no Listen
    Settings() json.RawMessage            // added in C4; opaque settings passed to the plugin as JSON
    PublishInbound(ctx context.Context, msg InboundMessage) error
    Media() MediaStore                    // first cut may be no-op
}
```

`Register()` still returns `[]plugin.Plugin`. A `seam: channel` plugin:

- must implement `Channel`;
- `Tools()` must be empty (if `Plugin` still has that method);
- Host routes by `Seam()` and does not adapt it into a tool.

The existing `hello-fs` `Plugin` shape does not need to be split in this slice. Do not churn the tool-plugin ABI just to add channel.

`PLUGIN-SPEC` forbids "starting a long-lived background service in a plugin to claim a port." This is the only channel exception: when it holds `channel.poll`, `Start` may run **outbound** long polling or an outbound WS client. `net.Listen` / `http.ListenAndServe` remain forbidden.

For `verify` with `seam: channel`:

| Forbidden | Reason |
|---|---|
| `net.Listen` / `http.ListenAndServe` / `ListenAndServeTLS` | Listen belongs to Host |
| `os.Open` / `exec.Command` bypassing Env | Same as existing plugins |
| import `internal/`, import eino | Same as existing plugins |
| Manifest includes `tools` | Wrong Consumer |
| grant includes `channel.webhook` / `channel.listen` / `channel.a2a` while the batch recipe enables it | Not allowed for the five plugins in this batch |
| import picoclaw or `.workspace` | Rewrite; do not depend on it |

Optional capabilities (typing, MessageEditor, Placeholder, MediaSender, WebhookHandler, StreamingCapable, TaskLifecycle, PipeServer) still use interfaces and are discovered by Host through type assertions. The adapter does not hold `*runtime.Service`.

Even when v1 Telegram and Host call each other in the **same process**, Inbound goes only through `Env.PublishInbound`. Moving the package to a `vivy channel` child process later then requires no adapter changes.

---

## 10. Recipe and Pack

Do not add a `channels:` recipe key for this batch. Name entries through the existing `plugins:` key:

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
providers:
  - openai
tools:
  - notes
  - filesystem
  - execute
  - ask-user
plugins:
  - plugins/hello-fs          # seam: tool-world
  - plugins/telegram          # seam: channel
  - plugins/dingtalk          # seam: channel
```

Rules:

- A plugin not written into the recipe does not exist in this generation. Do not scan `plugins/`.
- `pack` generates the same `Register()` that authors must not edit by hand:

```go
// Code generated by vivy-sdk pack. DO NOT EDIT.
func Register() []plugin.Plugin {
    return []plugin.Plugin{
        hellofs.New(),
        telegram.New(),
        dingtalk.New(),
    }
}
```

- `inspect` / `generation.json` list name, version, seam, grants, transport, source_ref, and tree_hash by seam. telegram is printed as a channel, not a tool.
- The species body committed on the main line is the complete body: `internal/generated/plugins/zz_register.go`
  registers all first-party channel plugins, so `just run`, the embedded-UI binary, and Docker have
  all ears out of the box. At build time, `pack` uses `-overlay` to replace the same file with the narrower
  combination selected by `--with` to assemble a minimal generation; the committed body remains complete.

Removing a channel plugin = delete a line from `plugins:` and pack again. Then run eval / promote. It is not deleting a runtime allowlist entry.

Developer experience:

```text
vivy-sdk verify plugins/telegram
vivy-sdk pack --with telegram --with dingtalk
vivy-sdk inspect-artifact dist/...
```

---

## 11. Configuration: Fixed Envelope, Opaque Settings

Configuration is valid if and only if it turns knobs for names **already compiled into this generation**. It cannot grow new organs.

```yaml
channels:
  telegram:
    enabled: true
    allow_from: ["telegram:123456"]
    token_env: TELEGRAM_BOT_TOKEN
    settings:                    # opaque to the kernel
      parse_mode: html
      proxy: "http://127.0.0.1:7890"
```

| Field | Decoder | Rule |
|---|---|---|
| Name (map key) | Host | Must appear in this generation's `Register()` with `seam: channel`; otherwise startup fails |
| `enabled` | Host | `false` = do not Start. inspect still displays compiled-in |
| `allow_from` | Host | Empty or omitted = reject Start. `"*"` is not allowed in the first cut |
| `token_env` | Host → `SecretResolver` | Must match `^[A-Z][A-Z0-9_]*$`. No literal token |
| `settings` | **The channel plugin** | Unknown fields fail-closed. If this generation's lib does not know a new field, upgrade the version and pack again |

Therefore an "optional configuration-file update" covers only:

- non-core fields already compiled into the decoder (proxy, parse_mode, placeholder copy);
- turning off an ear tonight (`enabled: false`);
- tightening / rewriting `allow_from` or changing the name in `token_env`.

It does not cover adding Discord, changing transport to webhook, adding a new event type to Host, or widening grants.

This follows the same philosophy as remote MCP and provider `env_key`: **configuration is knobs and remote dependencies, not a plugin system.**

Kernel `config.go` adds only the channel **envelope** (name, enabled, allow_from, token_env, opaque settings). Do not put `TelegramSettings` or `FeishuSettings` in `internal/config`.

Current frontend state (`UI-CHANNELS-BE`): seven-platform forms write to localStorage, the `allow_from` copy says "leave blank for no restriction," and the platform list includes email / neuro-link. When connecting the backend, change it to:

- toggle only **compiled-in** names;
- empty `allow_from` = reject startup;
- remove email / neuro-link from the addable list until corresponding plugins exist.

---

## 12. Inbound Path and Ledger

```text
platform Update
  → adapter normalizes InboundMessage / SenderInfo / parts
  → ChannelEnv.PublishInbound
  → Host: allow_from (empty = discard and record an audit entry; a channel that is not started cannot reach here)
  → Journal  channel.inbound
        {channel, peer, message_id, content_digest, bytes}
        never write tokens, raw secrets, or unbounded attachments
  → SessionMap.Ensure(channel, chat_id[, topic]) → Session (with provenance)
  → Message(role=user, source=channel, …)
  → Service.Run
  → live surface: typing / placeholder (does not enter the Journal)
  → run terminal state → Host → adapter.Send
```

NG-10: every new model-visible input requires a new event. Today, writing Telegram text as an ordinary `user` row makes replay look as if it came from the local UI.

Contract changes needed (after adoption, in a separate PR; do not smuggle implementation into this document):

- New `EventType`: `channel.inbound` (and optional `channel.started` / `channel.stopped` / `channel.lost`; the latter two may start as live-surface events)
- Add provenance to `domain.Message`: `source` (`ui` \| channel name), `peer` (bounded)
- Keep `run.started` as `additionalProperties: false`; do not put provenance into the old payload
- SQLite / Postgres migrations and conformance

The first cut does not do media, group triggers, or incremental streaming edits. If Telegram forums are encountered, append `topic_id` to the mapping key as picoclaw does, to avoid mixing contexts.

HITL: the kernel still decides all approval / question outcomes. The first cut dual-writes to the local UI; the channel only delivers text such as "there is a pending approval" (optional and may be cut later). A Telegram user directly deciding an approval = an ACP remote principal, not part of this contract.

---

## 13. Control Plane and Network

`:8787` remains loopback JSON-RPC. A Channel's `poll` is "contacting a provider" (the same category as MCP and model HTTP); it does not turn the local machine into a hosting entry point.

`channel.webhook` / `channel.listen` and `0.0.0.0` are product decisions that require an explicit revisit of PRD §5.0.1 / D-016. The five plugins in this batch have no webhook / listen. If NeuroLink or a webhook is added later:

- Host opens a second listen, loopback by default;
- public exposure or a tunnel must be explicitly enabled in configuration;
- the adapter must not Bind itself.

There is no need to implement complete CRUD for `channels/list` in the first cut. `species/inspect` can already list compiled-in items; configuration remains file-based. Add a read-only RPC later to avoid turning it into a runtime loader. Replacing the settings page's read/write layer belongs to `UI-CHANNELS-BE` and depends on Host landing.

---

## 14. The First Five Plugins

The product does not need picoclaw's 21 protocols; it needs a set of **domestic ears that can run overnight + two international ears**.
Rewrite all adapters from `.workspace/picoclaw` (**do not** import its modules) and connect them to the same Host.
The committed default `vivy.exe` still has `Register() = nil`. The following are **plugins that can be named in a recipe**, not parts welded into the daily body.

The first cut for every adapter is: private (or direct) text in / text out, `allow_from` fail-closed, `token_env`, no media, no group triggers, and no HITL proxy approval.
Groups / media / placeholder editing / streaming come later for each package and do not block Host.

### 14.1 Waves (by Transport and Product Risk, Not Name Recognition)

| Wave | Plugin | picoclaw source | Transport (code is authoritative) | Authentication | Why here |
|---|---|---|---|---|---|
| **0** | No protocol | — | — | — | Host + ledger + empty registry + SDK seam. Without this, porting is just copying a bot |
| **A** | `plugins/telegram` | `pkg/channels/telegram` | Outbound long-poll | Bot token | Minimal adapter to pin the ABI; CI uses fake updates and does not touch a real Bot |
| **A** | `plugins/dingtalk` | `pkg/channels/dingtalk` | Outbound Stream WS | `client_id` / `client_secret` | **First domestic ear**. No public network, QR, or 32-bit stub. Replies depend on the `session_webhook` delivered inbound |
| **B** | `plugins/feishu` | `pkg/channels/feishu` | Outbound Feishu SDK WS | `app_id` / `app_secret` | Domestic collaboration workhorse. The docs still say webhook, but **the implementation is WS**. 64-bit only; `encrypt_key` goes in lib `settings` |
| **B** | `plugins/qq` | `pkg/channels/qq` | Official Bot WS | `app_id` / `app_secret` | Domestic communities. It is a **QQ Open Platform bot**, not a personal account |
| **B** | `plugins/discord` | `pkg/channels/discord` | Gateway WS | Bot token | International community. **Do not** port `voice.go` / `pion/webrtc` / TTS detection |
| **Later** | `plugins/neurolink` | Diva `neuro_link.rs` (contract, not code source) | Host listen + local WS | Local binding | Heavyweight pipeline. Not in this batch or the settings page's addable list |
| **Later** | `plugins/a2a` | eino-ext/a2a models/transport (codec); Diva A2A research package (northbound adapter) | HTTP+JSON (off by default) | Bearer / skill allowlist | Task = Run. Do not treat `RegisterServerHandlers` as the gateway. Not in this batch |
| **Not in this batch** | wecom / weixin / onebot / email | — | — | — | Binding surface, personal accounts, a second body, and email are separate work |

Slack / LINE / Matrix retain their Diva 2026-08-18 retirement status and are not in this batch.

### 14.2 Recommended Two-Generation Recipes (Examples, Not the Default Body)

The "international proofing body" for development/evaluation (exists only when named explicitly):

```yaml
plugins:
  - plugins/telegram
  - plugins/discord
```

The "office body" for domestic overnight use:

```yaml
plugins:
  - plugins/dingtalk
  - plugins/feishu
  - plugins/qq
```

The resident may want only one of these. Do not weld all five into the default `just run` EXE for convenience.
`inspect` must show what this generation compiled in and whether telego or the lark SDK is missing.

### 14.3 First-Cut Scope of Each Package

| Package | Does | Explicitly does not do (later for this package) |
|---|---|---|
| telegram | Private-chat text, long-poll, proxy/`base_url` may go in settings | webhook, groups, media, command menus, full MarkdownV2 |
| dingtalk | Direct-chat text, Stream mode, save and use session webhook | Cards, media; do not turn it into a webhook text bot |
| feishu | Direct-chat text, WS events, `is_lark` domain switch | 32-bit, emoji, the public webhook mode described in the docs |
| qq | Direct/channel text (as reliably received through the official API) | Large-file base64, voice, personal accounts, OneBot |
| discord | DM / text-channel text, Message Content Intent | `voice.go`, WebRTC, the full slash-command suite, TTS |

### 14.4 Rewrite Rules (Relative to picoclaw)

- Borrow only: `Start`/`Stop`/`Send`, InboundContext/SenderInfo, error classification, and that platform's token usage.
- Do not borrow: `init()` blank-importing into Gateway, allowing an empty `allow_from`, per-channel self-built HTTP, or the kernel's `TelegramSettings` type.
- Each package's own `settings.go` decodes opaque yaml. Host sees only the envelope.
- One `vivy-plugin.json` per package. `pack` imports only what the recipe names. The Feishu SDK must not appear in the `go.mod` closure of a generation containing only telegram.

---

## 15. A2A / NeuroLink Reservations (Not Implemented in This Batch)

Both are heavyweight plugins on ChannelHost, not a new kernel and not Face / ACP.

**NeuroLink**

- Local WebSocket **server** for third-party pipelines (desktop companions, etc.)
- grant: `channel.listen`; Host owns the bind, loopback by default
- Protocol frames can be decided later; inbound still goes through `PublishInbound`, with chat/sender mapped to a channel Session
- Do not put it in a Telegram-style required-field card such as `channel_statuses`
- The settings page must not list "Add NeuroLink" before this plugin is compiled into the body

**A2A**

- Northbound agent interoperability. Baseline A2A v1.0 HTTP+JSON, off by default
- grant: `channel.a2a`
- `taskId` = existing Vivy `run_id`; do not create a second TaskStore semantic
- Agent Card is a capability declaration, not a permission system; real permissions remain Policy / Approval
- Requests must enter `Service.Run` and must not bypass the sandbox or approvals
- Protect outbound URLs against SSRF; treat remote responses as untrusted input

The envelope already reserves `task_id` / parts / stream for them in this batch. Do not let the five chat plugins turn these into private metadata first.

### 15.1 Relationship to Eino-Native A2A

Eino **core** (Vivy pin `github.com/cloudwego/eino v0.9.13`'s `adk.ChatModelAgent` + `Runner`) has no A2A wire protocol. In-process multi-agent behavior is `AgentAsTool` / DeepAgent, unrelated to channels, and is not touched in this batch.

Eino **native A2A** is in the extension package `github.com/cloudwego/eino-ext/a2a` (read-only reference, not a dependency in this batch; observed at `v0.0.1-alpha.13`). Split it into two layers; do not turn it into one "all-native package":

| Layer | What it is | Vivy |
|---|---|---|
| Protocol / codec | Agent Card, JSON-RPC, Task, Message parts, Stream (`models` + `transport`) | **Later `plugins/a2a` codec borrows this** |
| Example server | `extension/eino.RegisterServerHandlers(adk.Agent)`: self-built Hertz + default in-memory TaskStore, direct `adk.NewRunner().Run/Resume` | **Do not use as the product path** |

Correct layering (later C9, not implemented in this batch):

```text
A2A JSON-RPC / Agent Card          ← eino-ext/a2a models + transport
        ↓
plugins/a2a  (seam: channel)       ← independent go.mod; codec + capability declaration only
        ↓
ChannelHost                        ← allow_from, session, channel.inbound, Journal
        ↓
Service.Run → existing ADK Runner  ← Eino-native loop (currently in internal/runtime)
        ↓
Events back to Host → plugin Send  ← StreamResponse / Task state
```

Do not attach `RegisterServerHandlers` to Vivy's `adk.Agent`. That would create a separate TaskStore, bypass Journal / Policy / approvals / sandbox, add a second Hertz listen surface, and drag the alpha dependency and Hertz into the default body. The module's declared eino version also does not match Vivy's pin, so it cannot be a drop-in. `VIVY-PLUGIN-SPEC.md` already forbids plugins from importing `github.com/cloudwego/eino*`—if the A2A stack enters the body, it may appear only in the independent `plugins/a2a` go.mod and must not be visible in the default `just ci` closure.

The same applies to ACP: `eino-ext/acp` is also a direct `AgentEvent` protocol. The Vivy ACP proposal already rejects a second runtime; A2A does too.

The five chat plugins in this batch have zero Eino imports. C1–C8 do not need to change shape for A2A. C9 writes a separate capability proposal.

---

## 16. Acceptance for "Clean Removal"

After removing telegram from a generation, all of the following must hold for the **new EXE** (not the old process still running):

1. `Register()` contains no telegram
2. The recipe and dependency graph from `vivy-sdk inspect-artifact` contain no telegram / telego
3. After startup there is no telegram child process and no Telegram HTTP request
4. If `channels.telegram` remains in configuration, startup fails (the name is absent from the body) rather than silently ignoring it
5. The old Journal's `channel.inbound` **remains** (the ledger is genetic material; a generation removes an organ without deleting history)

Acceptance for `enabled: false` is different: telegram remains in the binary, inspect lists compiled-in + disabled, and the process does not poll.

Default `just ci` path: `go test ./...` does not compile the five channel modules, and the species `go.mod` contains no telego / discordgo / lark / DingTalk / botgo.

---

## 17. Boundary with Existing Plugins, MCP, Worker, and Face

```text
Kind A  Skill text             not compiled
Kind B  Capability source      tool / provider / tool-world / **channel**
Kind C  Generation EXE         the only loading action is pack
config  MCP address, env_key, channel envelope
worker  same-binary child run  not a plugin channel; a later channel child process copies its argv pattern
face    local mouth             another proposal; channel must not replace it
ACP     remote control          another proposal; not an inbound world
```

channel is the new Kind B seam; it is not Kind A, MCP, or a second EXE.

`config.tools.enabled` governs built-in tools. ADR-015: user tools packed in bypass this table. Channel is symmetric: the recipe determines the flesh, and envelope `enabled` determines whether it is awake. There is no `channels.allow` for launching external processes.

---

## 18. Key Decisions

1. **Host is the kernel, adapters are plugins.** Pluginizing Host would make `channel.inbound` drift with adapters, and the Journal would no longer be genetic material.
2. **All five in this batch go in `plugins/`, with `seam: channel`.** Do not add `channels/` or `RegisterChannels()`. inspect labels by seam.
3. **The default registry is empty.** It mirrors ADR-015. Daily `just run` / default `vivy.exe` has no ears.
4. **Configuration cannot invent a name absent from the body.** This is the root of cold plug/unplug.
5. **The kernel decodes only the envelope; plugins decode settings.** Otherwise every new protocol changes `internal/config`.
6. **Transport in this batch = poll (including outbound WS clients).** Revisit webhook / listen with a local-first stance.
7. **Write the contract along process boundaries; the first cut may be in-process.** Defer the crash domain; get the ABI right first.
8. **Empty `allow_from` is fail-closed.** A personal gateway does not speak to the whole world by default. The settings-page copy must change accordingly.
9. **Do not import picoclaw.** Rewrite the adapters and borrow the message model and capability interfaces.
10. **HITL remains local.** Treat a channel as an approver in a separate proposal.
11. **An independent `go.mod` is a hard requirement for this batch.** Large SDKs must not enter the default `just ci` closure.
12. **Fix the envelope shape in the first cut.** A2A / NeuroLink are later plugins, not later contract slots.
13. **Face / ACP remain independent.** The web UI is not `seam: channel`.
14. **Eino-native A2A = borrow the protocol, not `RegisterServerHandlers`.** The loop remains `Service.Run`; touch `eino-ext/a2a` only for the later codec.

---

## 19. Non-goals (This Contract)

- Change `sdk/plugin` or add event types while merging this document (that is a post-adoption implementation PR)
- Hot mounting / hot unloading / marketplace scanning
- The complete picoclaw protocol matrix, media pipeline, group triggers, or streaming placeholder editing
- Public webhook, Discord voice, WeChat personal accounts, an external OneBot bridge, or email
- WeCom QR binding surface
- Replacing the local UI with a channel
- Implementing A2A or NeuroLink
- Treating the Hertz example server / `RegisterServerHandlers(adk.Agent)` from `eino-ext/a2a` as the Vivy gateway
- Writing the five SDKs into the default `go.mod`

---

## 20. Implementation Slices (At Implementation Time)

The order is the dependency order. Each cut should be independently evaluable; unfinished work does not appear in the default EXE.

| Slice | Does | Success |
|---|---|---|
| C0 Contract | Adopt this document; cross-reference ASSEMBLY / PLUGIN-SPEC / GATEWAY | Documentation consistent, no code. **This slice** |
| C1 Ledger | `channel.inbound` event schema, Message provenance, storage migrations, conformance | `just ci`; no adapters |
| C2 SDK + empty registry + envelope configuration | `SeamChannel`, grants, `ChannelEnv`, verify forbids Listen / tools, pack can overlay non-tool plugins | No protocol deps; an empty pack list remains an empty body |
| C3 Host + fake channel plugin TCK | Capability discovery, fail-closed, PublishInbound → accounting → Run → Send | No real protocol |
| C4 `plugins/telegram` | Private-chat text polling; independent module; fail-closed allow_from | Candidate sends and receives text; default EXE still has no telego |
| C5 inspect / UI | inspect lists compiled-in vs enabled; settings page shows only names in the body; correct empty allow_from copy | Resident can see whether this generation has ears |
| C6 `plugins/dingtalk` | Stream; session webhook only in lib settings / runtime table | Domestic direct-chat text loop |
| C7 `plugins/feishu` + `qq` + `discord` | Three independent packages, named in three pack operations; Discord has no voice | Recipe can form an "office body" or "international body" |
| C8 Same-binary child process | `vivy channel --name <id>`; Host supervision | Kill one ear without breaking the ledger |
| C9 NeuroLink / A2A | Each requires an independent capability proposal + binding/auth surface. A2A: codec uses eino-ext/a2a models/transport; execution goes ChannelHost → `Service.Run` | No proposal means this slice stays closed; forbid binding the example server to ADK |

C0 is a documentation PR. The kernel changes only from C1 onward. Before C4, do not write `telego` into the species default `go.mod`. Evaluate each C4/C6/C7 package in one pack operation; do not "chain five SDKs into one PR."

The authoritative schedule, WBS, activity diagram, and Gantt are in `docs/TODO.md` §0.2 (decided 2026-08-30). The slices in this document are dependencies, not a calendar. Evolution tree: `VIVY-CHANNEL-EVOLUTION.md`. Child-AGENT assignments: `docs/plans/channel-epic/`.

---

## 21. Closed / Open Questions

Closed:

1. Whether built-in modules should have an independent `go.mod` — **yes** (hard requirement for this batch).
2. Whether to add `channels/` in this batch — **no**. Use `plugins/` + the existing `Register()`.
3. Whether the five adapters should enter the kernel — **no**. Pluginize all of them.
4. Super-channel boundary — Face / ACP remain independent; A2A / NeuroLink are later plugins on Host.
5. First-cut ABI thickness — fix the envelope and optional interfaces in the first cut; the five packages implement only required text.
6. Empty `allow_from` — fail-closed.
7. Eino-native A2A — protocol/codec may be reused; `RegisterServerHandlers(adk.Agent)` is not the product path. The loop remains `Service.Run`. The five plugins in this batch do not import Eino.

Still open (does not block C0; blocks later implementation or product work):

1. **Whether channel Session and local UI Session can be explicitly linked.** No in the first cut. Reopen with a product statement.
2. **`"*"` as `allow_from`.** No in the first cut. If public bots are wanted later, it must be explicit and appear in inspect.
3. **When to move `plugins/telegram` to `channels/telegram`.** Revisit when there are more first-party organs; the ABI does not change.
4. **When to remove the email / neuro-link cards from the settings page.** Remove them when integrating `UI-CHANNELS-BE`; C0 changes only the contract.

---

## 22. PR Plan (At Implementation Time)

### PR 1 — Adopt Contract

- Files: this document; `VIVY-ASSEMBLY.md`; `VIVY-PLUGIN-SPEC.md`; `SELF-EVOLVING-GATEWAY.md`; `VIVY-GATEWAY-AND-STUDIO.md`
- Dependencies: none
- No runtime code
- **This PR**

### PR 2 — Ledger Provenance

- Files: `internal/domain`, `schemas/events/`, `internal/storage/{sqlite,postgres,conformance}`
- Dependency: PR 1
- Add `channel.inbound` and Message provenance; old path uses `source=ui`

### PR 3 — SDK + Empty Registry + Envelope Configuration

- Files: `sdk/plugin`, `sdk/internal` verify/pack (support independent-module channel plugins), `internal/config`, `internal/pluginhost` routing
- Dependency: PR 2
- `pack` can overlay `seam: channel` into the existing `Register()`; the default body remains empty

### PR 4 — Minimal ChannelHost Loop (Fake Plugin)

- Files: Host under `internal/`, Session mapping, test fake channel plugin
- Dependency: PR 3
- Unit test: PublishInbound → accounting → Run; empty allow_from is rejected

### PR 5 — `plugins/telegram`

- Files: `plugins/telegram/` (independent go.mod), pack recipe example
- Dependency: PR 4
- Manual/isolated tests of the candidate EXE against the real Bot API; the default `just ci` path still does not link telego

### PR 6 — inspect and Settings Page

- Files: inspect RPC, `ui/` settings (replace the localStorage read/write layer; remove platforms not compiled in; correct allow_from copy)
- Dependency: PR 5
- Development verification uses `http://127.0.0.1:3015`

### PR 7 — `plugins/dingtalk`

- Files: `plugins/dingtalk/`
- Dependency: PR 4 (may run in parallel with PR 5, but Host must merge first)
- Keep the session webhook in the adapter

### PR 8 — `plugins/feishu`

- Files: `plugins/feishu/` (64-bit implementation only; 32-bit compilation must fail with a clear error)
- Dependency: PR 4
- WS events; do not implement public webhook

### PR 9 — `plugins/qq`

- Files: `plugins/qq/`
- Dependency: PR 4
- Official bot API, text only

### PR 10 — `plugins/discord` (No Voice)

- Files: `plugins/discord/` (do not copy `voice.go`)
- Dependency: PR 4
- This plugin's `go.mod` must not contain `pion/webrtc`

### PR 11 — Same-Binary Channel Child Process

- Files: `cmd/vivy` argv, Host supervision, failure classification
- Dependency: at least one real adapter (PR 5 or PR 7)
- Contract unchanged; move the adapter

NeuroLink / A2A each require an independent proposal and are not in this plan.

---

## 23. One Sentence (Repeated)

> **The recipe is the tree. A Channel plugin is one row on the tree.**
> A live process does not grow a new row. A new row appears only in the next-generation EXE.
> Host is the environment; the plugin is the flesh; yaml is the knob.
> Changing yaml is not a generation change; only a generation change can make telego disappear from the binary.
> All five ears are plugins. The default body is deaf.
