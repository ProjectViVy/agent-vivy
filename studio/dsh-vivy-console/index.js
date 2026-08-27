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
// Scope (2026-08-27): the development loop only — backend + frontend dev
// lifecycle + unified logs. Studio distribution (pack/eval/release/
// install/rollback) is deliberately NOT in the Studio UI; it stays on the
// vivy-sdk / vivy-studio.exe command line.
//
// Air gap: all backend data lives under
// <root>/data/studio-home/vivy-console (Studio's own scratch). The
// production Journal (data/vivy.db, data/demo/, data/workspaces/) is never
// touched (ST-2).
//
// Routes registered on the Studio webServer:
//   *    /vivy-console/api/*          -> JSON API (backend + frontend + logs)

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
    "  mock: true",
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
      env: { ...process.env, VIVY_CONFIG: cfgPath },
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
    message: `已启动 (PID ${child.pid})，监听 ${currentAddr}（纯 API 后端 · vivy_headless · mock 模式，数据隔离）`,
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
    default:
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
    "vivy-console: backend + frontend JSON API",
  )
  ctx.effect(
    () => dispose,
    "vivy-console: stop backend + frontend dev child processes",
  )
}