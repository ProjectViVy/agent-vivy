// Vivy Studio first-party console (Host half).
//
// Runs as a profile bundle plugin inside the Studio server process (real
// Node, no vm sandbox). It manages the two processes of the unified
// development loop — one model only, matching AGENTS.md's split pair:
//   * the backend: a PURE-API vivy binary with NO embedded frontend
//     (go build -tags vivy_headless; serves only the /rpc control plane),
//     compiled on demand from the workspace source into Studio scratch
//   * the frontend: the DEV dev server (pnpm dev in ui/, Vite :3015) —
//     the sole user-facing app; its /rpc proxy targets the managed backend
// — and exposes both processes' logs as one unified timeline.
//
// There is intentionally no "embedded UI" debugging: the backend build
// carries no UI (vivy_headless), so the /vivy-web same-origin facade, the
// frontend-debug bridge (hook.js) and the Studio-origin /vivy-config.json
// route are gone. Frontend debugging happens in the normal browser at
// http://127.0.0.1:3015.
//
// Scope (2026-08-27): the console has two clearly separated concerns —
//   * 总控台 (development loop): backend + frontend dev lifecycle + logs
//   * 打包与版本 (packaging & version management): the Studio distribution
//     ledger and actions (pack/eval/release/reject/install/rollback/
//     inspect), driven through vivy-studio.exe, NOT by starting any dev
//     process. Release keeps the human gate (NG-25).
//
// Air gap: all backend data lives under
// <root>/data/studio-home/vivy-console (Studio's own scratch). The
// production Journal (data/vivy.db, data/demo/, data/workspaces/) is never
// touched (ST-2).
//
// Routes registered on the Studio webServer:
//   *    /vivy-console/api/*          -> JSON API (backend + frontend + logs)
//   *    /vivy-console/api/lifecycle/*-> packaging & version management
//                                        (vivy-studio.exe: pack/eval/release/
//                                        reject/install/rollback + ledger)
//
// The lifecycle surface is deliberately a SEPARATE concern from the dev
// loop: it never starts, stops, or inspects the managed backend/frontend
// processes. Distribution stays behind the human gate (release needs an
// explicit UI confirmation which forwards --actor human --yes).

import { spawn, spawnSync } from "node:child_process"
import {
  appendFileSync,
  closeSync,
  existsSync,
  mkdirSync,
  openSync,
  readSync,
  statSync,
  writeFileSync,
} from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import net from "node:net"

export const name = "vivy-console"
export const inject = ["webServer"]

const here = dirname(fileURLToPath(import.meta.url))

// Workspace root: the Studio server is launched from the repository root
// (launch-vivy-studio.ps1 and boot-studio.cmd both `cd` there), so cwd is
// the canonical root.
const root = process.cwd().replace(/[\\/]+$/, "")
const consoleDir = join(root, "data", "studio-home", "vivy-console")
const cfgPath = join(consoleDir, "config.yaml")
const outLogPath = join(consoleDir, "gateway.out.log")
const errLogPath = join(consoleDir, "gateway.err.log")
const feOutLogPath = join(consoleDir, "frontend.out.log")
const feErrLogPath = join(consoleDir, "frontend.err.log")
const uiDir = join(root, "ui")

const norm = (p) => String(p || "").replace(/\\/g, "/")

const apiPrefix = "/vivy-console/api"
const FE_PORT = 3015
const DEFAULT_ADDR = "127.0.0.1:8787"

let exeOverride = "" // explicit backend binary (overrides auto-compile)
let currentAddr = DEFAULT_ADDR
let child = null // ChildProcess of the managed backend, null once exited
let startedAtMs = 0
let feChild = null // ChildProcess of the Vite dev server, null once exited
let feStartedAtMs = 0
let studioPort = 0

// Vivy Code (TUI face) — a separate operator surface. Never mixed into the
// backend/frontend one-click lifecycle: it only opens a dedicated console
// window for `vivy tui --demo` (or --plain). No shared process tree with
// the gateway or Vite.
let codeOpenedAtMs = 0
let codeLastRoot = ""
let codeLastMode = ""
let codeLastMessage = ""

// ---- lifecycle jobs (packaging & version management; one concurrent) ----
const MAX_JOB_LINES = 500
const jobs = new Map()
let jobSeq = 0

// ---- backend binary plan ----
//
// The managed backend must be a PURE-API binary (no embedded frontend). The
// default is the vivy_headless build compiled into Studio scratch; explicit
// EXE override or VIVY_HEADLESS_EXE may supply a prebuilt headless binary.
function backendPlan() {
  if (exeOverride) return { exe: norm(exeOverride), autoBuild: false }
  const envExe = String(process.env.VIVY_HEADLESS_EXE || "").trim()
  if (envExe && existsSync(envExe)) return { exe: norm(envExe), autoBuild: false }
  return { exe: norm(join(consoleDir, "vivy-backend.exe")), autoBuild: true }
}

// The distribution tool (packaging & version management). Built by
// `just studio`; may be pinned via VIVY_STUDIO.
function resolveStudioExe() {
  const candidates = []
  if (process.env.VIVY_STUDIO) candidates.push(process.env.VIVY_STUDIO)
  candidates.push(join(root, "vivy-studio.exe"))
  for (const candidate of candidates) {
    if (candidate && existsSync(candidate)) return norm(candidate)
  }
  return ""
}

function buildBackend(exe) {
  // Incremental go build of the headless backend (warm cache is fast). The
  // output streams into the backend log so the unified timeline shows it.
  return new Promise((resolve) => {
    const args = ["build", "-tags", "vivy_headless", "-o", exe, "./cmd/vivy"]
    const proc = spawn("go", args, {
      cwd: root,
      env: process.env,
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    })
    let tailOut = ""
    let tailErr = ""
    let settled = false
    const timer = setTimeout(() => {
      if (settled) return
      settled = true
      try {
        proc.kill()
      } catch {
        /* best effort */
      }
      resolve({ ok: false, tail: "编译超时（go build 超过 15 分钟）" })
    }, 15 * 60 * 1000)
    proc.stdout.on("data", (chunk) => {
      const text = chunk.toString()
      tailOut = (tailOut + text).slice(-4000)
      try {
        appendFileSync(outLogPath, text)
      } catch {
        /* log file unavailable — keep compiling */
      }
    })
    proc.stderr.on("data", (chunk) => {
      const text = chunk.toString()
      tailErr = (tailErr + text).slice(-4000)
      try {
        appendFileSync(errLogPath, text)
      } catch {
        /* log file unavailable — keep compiling */
      }
    })
    proc.on("error", (error) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      resolve({ ok: false, tail: error instanceof Error ? error.message : String(error) })
    })
    proc.on("close", (code) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      const tail = (tailErr.trim() || tailOut.trim()).slice(-1200)
      resolve(code === 0 ? { ok: true, tail } : { ok: false, tail: tail || `exit ${code}` })
    })
  })
}

function portInUse(port) {
  return new Promise((resolve) => {
    const sock = net.connect({ port, host: "127.0.0.1" })
    sock.once("connect", () => {
      sock.destroy()
      resolve(true)
    })
    sock.once("error", () => resolve(false))
  })
}

async function probePort() {
  if (!(await portInUse(8787))) return 8787
  let port = 8790
  while (await portInUse(port)) port += 1
  return port
}

function readLogs() {
  // Unified timeline: backend lines + frontend (Vite dev) lines, each
  // tagged with its source so the client renders one feed with source
  // chips and filters by source.
  const tail = (path, maxBytes) => {
    try {
      const size = statSync(path).size
      if (size <= 0) return []
      const readSize = Math.min(size, maxBytes)
      const buffer = Buffer.alloc(readSize)
      const fd = openSync(path, "r")
      let bytes = 0
      try {
        bytes = readSync(fd, buffer, 0, readSize, size - readSize)
      } finally {
        closeSync(fd)
      }
      const text = buffer.subarray(0, bytes).toString("utf8")
      const lines = text.split(/\r?\n/)
      if (text.endsWith("\n")) lines.pop()
      return lines
    } catch {
      return []
    }
  }
  const gwOut = tail(outLogPath, 512 * 1024)
  const gwErr = tail(errLogPath, 256 * 1024)
  const feOut = tail(feOutLogPath, 512 * 1024)
  const feErr = tail(feErrLogPath, 256 * 1024)
  const gwLines = [...gwOut, ...gwErr.map((line) => (line ? "[err] " + line : line))]
  const feLines = [...feOut, ...feErr.map((line) => (line ? "[err] " + line : line))]
  return {
    lines: [
      ...gwLines.slice(-300).map((line) => ({ src: "backend", text: line })),
      ...feLines.slice(-300).map((line) => ({ src: "frontend", text: line })),
    ],
    backend: {
      logPath: norm(outLogPath),
      running: child !== null && child.exitCode === null,
    },
    frontend: {
      logPath: norm(feOutLogPath),
      running: feChild !== null && feChild.exitCode === null,
    },
  }
}

function findExternalVivy() {
  // Any vivy.exe not managed by this plugin (someone else's gateway).
  try {
    const result = spawnSync(
      "tasklist",
      ["/FI", "IMAGENAME eq vivy.exe", "/FO", "CSV", "/NH"],
      { encoding: "utf8", timeout: 10000, windowsHide: true },
    )
    if (result.status !== 0) return null
    const lines = (result.stdout || "")
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
    if (lines.length === 0) return null
    // tasklist CSV: "vivy.exe","1234","Console","1","12,345 K"
    const match = /"vivy\.exe","(\d+)"/.exec(lines[0])
    if (!match) return null
    return { pid: parseInt(match[1], 10) }
  } catch {
    return null
  }
}

function prepare(port) {
  mkdirSync(join(consoleDir, "data", "workspaces"), { recursive: true })
  mkdirSync(join(consoleDir, "data", "skills"), { recursive: true })
  const d = norm(consoleDir)
  // No allowed_origins needed: the headless backend has no UI of its own
  // and the Vite dev UI talks to /rpc same-origin through its own proxy.
  // Real-provider path only (2026-08-29): runtime.mock was removed from
  // product Config. Keys live under this scratch user home / settings.yaml
  // (or a frozen ENV session) — never a mock provider.
  const cfg = [
    "server:",
    `  addr: "127.0.0.1:${port}"`,
    "storage:",
    "  backend: sqlite",
    "  sqlite:",
    `    path: "${d}/data/vivy.db"`,
    "providers:",
    "  active: openai",
    `  bundle_dir: "${norm(root)}/fixtures/provider"`,
    "  openai:",
    "    env_key: OPENAI_API_KEY",
    "    default_model: gpt-4o-mini",
    "  anthropic:",
    "    env_key: ANTHROPIC_API_KEY",
    "    default_model: claude-sonnet-4-5",
    "runtime:",
    `  workspace_root: "${d}/data/workspaces"`,
    `  skills_root: "${d}/data/skills"`,
    "tools:",
    "  enabled:",
    "    - echo_info",
    "  approval:",
    "    expiration: 5m",
    "",
  ].join("\n")
  writeFileSync(cfgPath, cfg, "utf8")
}

async function startBackend() {
  if (child !== null && child.exitCode === null) {
    return { ok: false, message: "后端已在运行（本控制台管理）" }
  }
  const external = findExternalVivy()
  if (external && external.pid !== (child && child.pid)) {
    return { ok: false, message: `检测到外部 vivy 进程 (PID ${external.pid})，请先停止` }
  }
  const plan = backendPlan()
  if (!existsSync(dirname(plan.exe))) {
    return { ok: false, message: `后端目录不存在：${norm(dirname(plan.exe))}` }
  }
  if (plan.autoBuild) {
    const built = await buildBackend(plan.exe)
    if (!built.ok) {
      return {
        ok: false,
        message: `后端编译失败（go build -tags vivy_headless ./cmd/vivy）：${built.tail}`,
      }
    }
  }
  const port = await probePort()
  prepare(port)
  currentAddr = `127.0.0.1:${port}`
  const outFd = openSync(outLogPath, "a")
  const errFd = openSync(errLogPath, "a")
  try {
    child = spawn(plan.exe, [], {
      cwd: dirname(plan.exe) || root,
      env: {
        ...process.env,
        VIVY_CONFIG: cfgPath,
        // Keep settings/Journal under Studio scratch (ST-2 air gap).
        VIVY_USER_HOME: join(consoleDir, "data"),
      },
      stdio: ["ignore", outFd, errFd],
      windowsHide: true,
    })
  } catch (error) {
    closeSync(outFd)
    closeSync(errFd)
    child = null
    return {
      ok: false,
      message: "启动失败: " + (error instanceof Error ? error.message : String(error)),
    }
  }
  startedAtMs = Date.now()
  child.on("exit", () => {
    child = null
  })
  child.on("error", () => {
    child = null
  })
  return {
    ok: true,
    pid: child.pid,
    message: `已启动 (PID ${child.pid})，监听 ${currentAddr}（纯 API 后端 · vivy_headless · 真实 provider · 数据隔离）`,
  }
}

async function stopBackend() {
  if (child !== null && child.exitCode === null) {
    const pid = child.pid
    child.kill()
    child = null
    return { ok: true, message: `已停止 (PID ${pid})` }
  }
  const external = findExternalVivy()
  if (!external) return { ok: false, message: "后端未在运行" }
  try {
    spawnSync("taskkill", ["/PID", String(external.pid), "/T", "/F"], {
      windowsHide: true,
      timeout: 10000,
    })
  } catch {
    /* best effort */
  }
  return { ok: true, message: `已停止外部进程 (PID ${external.pid})` }
}

async function status() {
  const plan = backendPlan()
  const managed = child !== null && child.exitCode === null
  let external = null
  if (!managed) external = findExternalVivy()
  const port = parseInt(currentAddr.split(":").pop(), 10)
  const listening = await portInUse(port)
  const proc = managed
    ? { pid: child.pid, startedAtMs }
    : external
      ? { pid: external.pid, startedAtMs: 0 }
      : null
  return {
    running: !!proc,
    managed,
    pid: proc ? proc.pid : null,
    startedAtMs: proc ? proc.startedAtMs : null,
    addr: currentAddr,
    listening,
    exe: plan.exe,
    autoBuild: plan.autoBuild,
    configPath: norm(cfgPath),
    logPath: norm(outLogPath),
    isolated: true,
    studioPort,
    frontend: await frontendStatus(),
    code: codeStatus(),
  }
}

// ---- frontend dev server (Vite, ui/) ----

async function frontendStatus() {
  const managed = feChild !== null && feChild.exitCode === null
  const listening = await portInUse(FE_PORT)
  return {
    running: managed,
    managed,
    pid: managed ? feChild.pid : null,
    startedAtMs: managed ? feStartedAtMs : null,
    addr: `127.0.0.1:${FE_PORT}`,
    listening,
    cwd: norm(uiDir),
    command: "pnpm dev",
    logPath: norm(feOutLogPath),
    dirReady: existsSync(join(uiDir, "package.json")),
    url: `http://127.0.0.1:${FE_PORT}/`,
    rpcTarget: `http://${currentAddr}`,
  }
}

async function startFrontend() {
  if (feChild !== null && feChild.exitCode === null) {
    return { ok: false, message: "前端 dev server 已在运行（本控制台管理）" }
  }
  if (!existsSync(join(uiDir, "package.json"))) {
    return { ok: false, message: `未找到 ui/package.json（${norm(uiDir)}）` }
  }
  if (await portInUse(FE_PORT)) {
    return { ok: false, message: `端口 ${FE_PORT} 已被占用（可能外部 vite 已在运行）` }
  }
  const outFd = openSync(feOutLogPath, "a")
  const errFd = openSync(feErrLogPath, "a")
  try {
    feChild = spawn("pnpm", ["dev"], {
      cwd: uiDir,
      shell: process.platform === "win32",
      // Vite proxies /rpc to the managed backend (ui/vite.config.ts reads
      // VIVY_BACKEND_ADDR); the URL always matches the gateway's live port.
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${currentAddr}` },
      stdio: ["ignore", outFd, errFd],
      windowsHide: true,
    })
  } catch (error) {
    closeSync(outFd)
    closeSync(errFd)
    feChild = null
    return {
      ok: false,
      message: "启动失败: " + (error instanceof Error ? error.message : String(error)),
    }
  }
  feStartedAtMs = Date.now()
  feChild.on("exit", () => {
    feChild = null
  })
  feChild.on("error", () => {
    feChild = null
  })
  return {
    ok: true,
    pid: feChild.pid,
    message: `前端 dev server 已启动 (PID ${feChild.pid})，监听 ${FE_PORT}（Vite，代理 /rpc → ${currentAddr}）`,
  }
}

async function stopFrontend() {
  if (feChild !== null && feChild.exitCode === null) {
    const pid = feChild.pid
    // On Windows the child is cmd.exe (shell: true); tree-kill so the Vite
    // process underneath cannot be orphaned.
    if (process.platform === "win32") {
      try {
        spawnSync("taskkill", ["/PID", String(pid), "/T", "/F"], {
          windowsHide: true,
          timeout: 10000,
        })
      } catch {
        /* best effort */
      }
    } else {
      try {
        feChild.kill("SIGTERM")
      } catch {
        /* best effort */
      }
    }
    feChild = null
    return { ok: true, message: `前端 dev server 已停止 (PID ${pid})` }
  }
  return { ok: false, message: "前端 dev server 未在运行" }
}

// ---- Vivy Code (TUI development panel) ----
//
// Deliberately NOT part of the backend/frontend one-click path. Opens a
// dedicated OS console window for the Crush-style TUI shell. Source may
// live on this checkout or a sibling worktree that already has internal/tui.

function codeSourceReady(dir) {
  return (
    existsSync(join(dir, "cmd", "vivy", "tui.go")) &&
    existsSync(join(dir, "internal", "tui", "view", "model.go"))
  )
}

function resolveCodeRoot() {
  const pinned = String(process.env.VIVY_CODE_ROOT || "").trim()
  const candidates = []
  if (pinned) candidates.push(pinned)
  candidates.push(root)
  // Feature worktree cut for the TUI skeleton (parallel-lane isolation).
  candidates.push(join(root, "..", "agent-vivy-tui-crush"))
  for (const candidate of candidates) {
    if (candidate && codeSourceReady(candidate)) return norm(candidate)
  }
  return ""
}

function codeStatus() {
  const codeRoot = resolveCodeRoot()
  return {
    ready: !!codeRoot,
    root: codeRoot || "",
    mode: codeLastMode || "demo",
    openedAtMs: codeOpenedAtMs || null,
    lastMessage: codeLastMessage || "",
    command: "go run ./cmd/vivy tui --demo",
    note: "独立控制台窗口；不启动/停止后端或前端，不进入一键启停",
  }
}

function startCode(body) {
  const mode = body && body.mode === "plain" ? "plain" : "demo"
  const codeRoot = resolveCodeRoot()
  if (!codeRoot) {
    return {
      ok: false,
      message:
        "未找到 Vivy Code 源码（需要 cmd/vivy/tui.go 与 internal/tui/view）。可设置 VIVY_CODE_ROOT，或使用已合入 TUI 的 worktree（例如 ../agent-vivy-tui-crush）",
    }
  }
  const tuiArgs = mode === "plain" ? "tui --plain" : "tui --demo"
  const cmdline = `go run ./cmd/vivy ${tuiArgs}`
  try {
    if (process.platform === "win32") {
      // `start "title" cmd /k ...` opens a visible, dedicated console that is
      // not Studio's stdio and not the backend/frontend log pipes.
      const childProc = spawn(
        "cmd.exe",
        ["/c", "start", "Vivy Code", "cmd.exe", "/k", cmdline],
        {
          cwd: codeRoot,
          detached: true,
          stdio: "ignore",
          windowsHide: false,
          env: process.env,
        },
      )
      childProc.unref()
    } else {
      const childProc = spawn("go", ["run", "./cmd/vivy", ...(mode === "plain" ? ["tui", "--plain"] : ["tui", "--demo"])], {
        cwd: codeRoot,
        detached: true,
        stdio: "ignore",
        env: process.env,
      })
      childProc.unref()
    }
  } catch (error) {
    return {
      ok: false,
      message: "打开失败: " + (error instanceof Error ? error.message : String(error)),
    }
  }
  codeOpenedAtMs = Date.now()
  codeLastRoot = codeRoot
  codeLastMode = mode
  codeLastMessage = `已在独立窗口打开 Vivy Code（${mode}）@ ${codeRoot}`
  return {
    ok: true,
    message: codeLastMessage,
    root: codeRoot,
    mode,
    openedAtMs: codeOpenedAtMs,
  }
}

// ---- packaging & version management (vivy-studio.exe) ----
//
// This surface is separate from the dev loop: it never starts/stops the
// managed backend or frontend. It reads the Studio ledger and runs the
// distribution actions through vivy-studio.exe on the pinned worktree.
// Release forwards `--actor human --yes` only after an explicit UI
// confirmation (human gate, NG-25).

function lifecycleList(kind) {
  const exe = resolveStudioExe()
  if (!exe) {
    return Promise.resolve({ ok: false, message: "未找到 vivy-studio.exe（先运行 just studio）" })
  }
  // Worktrees live under the workspace subcommand, not `list`.
  const argv = kind === "worktrees" ? ["--worktree", root, "workspace", "list"] : ["--worktree", root, "list", kind]
  return new Promise((resolve) => {
    const proc = spawn(exe, argv, {
      windowsHide: true,
      env: process.env,
    })
    let stdout = ""
    let stderr = ""
    proc.stdout.on("data", (chunk) => {
      stdout += chunk.toString("utf8")
    })
    proc.stderr.on("data", (chunk) => {
      stderr += chunk.toString("utf8")
    })
    proc.on("error", (error) => {
      resolve({ ok: false, message: "vivy-studio: " + error.message })
    })
    proc.on("close", (code) => {
      if (code !== 0) {
        resolve({ ok: false, message: stderr.trim() || `vivy-studio exited ${code}` })
        return
      }
      try {
        resolve({ ok: true, value: JSON.parse(stdout) })
      } catch (error) {
        resolve({ ok: false, message: "解析失败: " + (error instanceof Error ? error.message : error) })
      }
    })
  })
}

function lifecycleRun(body) {
  const exe = resolveStudioExe()
  if (!exe) return { ok: false, message: "未找到 vivy-studio.exe（先运行 just studio）" }
  for (const job of jobs.values()) {
    if (job.status === "running") {
      return { ok: false, message: "已有生命周期任务在运行（并发上限 1）" }
    }
  }
  const action = typeof body.action === "string" ? body.action : ""
  const args = ["--worktree", root, action]
  const push = (value) => {
    if (typeof value === "string" && value !== "") args.push(value)
  }
  switch (action) {
    case "pack": {
      const withList = Array.isArray(body.with) ? body.with : []
      for (const name of withList) {
        push("--with")
        push(name)
      }
      if (body.out) {
        push("--out")
        push(body.out)
      }
      break
    }
    case "eval":
      push("--candidate")
      push(body.candidate)
      if (body.baseline) {
        push("--baseline")
        push(body.baseline)
      }
      if (body.suite) {
        push("--suite")
        push(body.suite)
      }
      break
    case "release": {
      // Human gate (NG-25): only an explicit UI confirmation may forward
      // --actor human --yes. The CLI refuses any other actor / missing --yes.
      if (body.confirm !== true) {
        return { ok: false, message: "发布必须由人显式确认（NG-25）" }
      }
      push("--generation")
      push(body.generation)
      if (body.eval) {
        push("--eval")
        push(body.eval)
      }
      args.push("--actor", "human", "--yes")
      break
    }
    case "reject":
      push("--generation")
      push(body.generation)
      break
    case "install":
      push("--release")
      push(body.release)
      if (body.target) {
        push("--target")
        push(body.target)
      }
      break
    case "rollback":
      if (body.target) {
        push("--target")
        push(body.target)
      }
      break
    case "inspect":
      if (body.target) {
        push("--target")
        push(body.target)
      }
      break
    default:
      return { ok: false, message: "未知生命周期动作: " + action }
  }
  const id = "job_" + String(++jobSeq)
  const job = {
    id,
    action,
    status: "running",
    output: [],
    exitCode: null,
    child: null,
    startedAtMs: Date.now(),
  }
  jobs.set(id, job)
  const proc = spawn(exe, args, { windowsHide: true, env: process.env })
  job.child = proc
  const onData = (chunk) => {
    for (const line of chunk.toString("utf8").split(/\r?\n/)) {
      if (line === "") continue
      job.output.push(line)
    }
    if (job.output.length > MAX_JOB_LINES) {
      job.output.splice(0, job.output.length - MAX_JOB_LINES)
    }
  }
  proc.stdout.on("data", onData)
  proc.stderr.on("data", onData)
  proc.on("error", (error) => {
    job.status = "failed"
    job.exitCode = -1
    job.child = null
    job.output.push("spawn error: " + error.message)
  })
  proc.on("close", (code) => {
    job.status = code === 0 ? "ok" : "failed"
    job.exitCode = code
    job.child = null
  })
  return { ok: true, id, action }
}

function jobSnapshot(job) {
  return {
    id: job.id,
    action: job.action,
    status: job.status,
    exitCode: job.exitCode,
    startedAtMs: job.startedAtMs,
    output: job.output.slice(-MAX_JOB_LINES),
  }
}

// ---- HTTP plumbing ----

function readBody(req) {
  return new Promise((resolve) => {
    let data = ""
    req.on("data", (chunk) => {
      data += chunk
      if (data.length > 64 * 1024) req.destroy()
    })
    req.on("end", () => {
      try {
        resolve(data ? JSON.parse(data) : {})
      } catch {
        resolve({})
      }
    })
    req.on("error", () => resolve({}))
  })
}

function json(res, payload) {
  const body = JSON.stringify(payload)
  res.writeHead(200, {
    "content-type": "application/json; charset=utf-8",
    "cache-control": "no-store",
  })
  res.end(body)
}

async function handleConsoleApi(req, res, route) {
  const method = req.method || "GET"
  const body = method === "POST" ? await readBody(req) : {}
  let payload
  switch (`${method} ${route}`) {
    case "GET /status":
      payload = await status()
      break
    case "GET /logs":
      payload = readLogs()
      break
    case "GET /resolve":
      payload = {
        exe: backendPlan().exe,
        autoBuild: backendPlan().autoBuild,
        studio: resolveStudioExe(),
        configPath: norm(cfgPath),
        logPath: norm(outLogPath),
        root: norm(root),
      }
      break
    case "POST /start":
      payload = await startBackend()
      break
    case "POST /stop":
      payload = await stopBackend()
      break
    case "POST /restart":
      await stopBackend()
      payload = await startBackend()
      break
    case "POST /setExe":
      exeOverride = body && typeof body.path === "string" ? body.path.trim() : ""
      payload = { ok: true, exe: exeOverride }
      break
    case "GET /frontend/status":
      payload = await frontendStatus()
      break
    case "POST /frontend/start":
      payload = await startFrontend()
      break
    case "POST /frontend/stop":
      payload = await stopFrontend()
      break
    case "POST /frontend/restart":
      await stopFrontend()
      payload = await startFrontend()
      break
    case "GET /code/status":
      payload = codeStatus()
      break
    case "POST /code/open":
      payload = startCode(body)
      break
    default:
      if (method === "GET" && route.startsWith("/lifecycle/list")) {
        const kind = new URL(req.url || "/", "http://x").searchParams.get("kind") || "generations"
        payload = await lifecycleList(kind)
        break
      }
      if (method === "POST" && route === "/lifecycle/run") {
        payload = lifecycleRun(body)
        break
      }
      if (method === "GET" && route.startsWith("/lifecycle/jobs/")) {
        const id = route.slice("/lifecycle/jobs/".length)
        const job = jobs.get(id)
        payload = job ? { ok: true, job: jobSnapshot(job) } : { ok: false, message: "任务不存在" }
        break
      }
      res.writeHead(404, { "content-type": "application/json" })
      res.end(JSON.stringify({ ok: false, message: `no route: ${method} ${route}` }))
      return
  }
  json(res, payload)
}

async function handleConsoleRoute(req, res) {
  try {
    const url = new URL(req.url || "/", "http://x")
    if (url.pathname.startsWith(apiPrefix)) {
      await handleConsoleApi(req, res, url.pathname.slice(apiPrefix.length) || "/")
      return
    }
    res.writeHead(404, { "content-type": "application/json" })
    res.end(JSON.stringify({ ok: false, message: "not found" }))
  } catch (error) {
    json(res, { ok: false, message: error instanceof Error ? error.message : String(error) })
  }
}

function dispose() {
  if (child !== null) {
    try {
      child.kill()
    } catch {
      /* best effort */
    }
    child = null
  }
  if (feChild !== null) {
    try {
      feChild.kill()
    } catch {
      /* best effort */
    }
    feChild = null
  }
  for (const job of jobs.values()) {
    if (job.child !== null) {
      try {
        job.child.kill()
      } catch {
        /* best effort */
      }
      job.child = null
      job.status = "failed"
      job.output.push("插件卸载，任务已终止")
    }
  }
}

export function apply(ctx) {
  studioPort = typeof ctx.webServer.port === "number" ? ctx.webServer.port : 3090
  ctx.effect(
    () =>
      ctx.webServer.register({
        kind: "prefix",
        path: "/vivy-console",
        handler: handleConsoleRoute,
      }),
    "vivy-console: backend + frontend JSON API + packaging/version API",
  )
  ctx.effect(
    () => dispose,
    "vivy-console: stop backend + frontend dev children and lifecycle jobs",
  )
}