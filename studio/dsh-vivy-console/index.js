// Vivy Studio first-party console (Host half).
//
// Runs as a profile bundle plugin inside the Studio server process (real
// Node, no vm sandbox). Supersedes dsh-vivy-debugger: it manages two child
// processes of the development loop —
//   * the vivy.exe gateway (status/logs/start/stop/restart/exe override)
//   * the Vite frontend dev server (pnpm dev in ui/, :3015)
// — serves a same-origin facade of the VIVY WEB UI at /vivy-web/ with a
// frontend-debug bridge injected into the proxied HTML, and exposes both
// processes' logs as one unified timeline.
//
// Scope (2026-08-27): the development loop only — gateway + frontend dev +
// logs + VIVY WEB debugging. Studio distribution (pack/eval/release/install/
// rollback) is deliberately NOT in the Studio UI; it stays on the vivy-sdk /
// vivy-studio.exe command line.
//
// Air gap: all gateway data lives under
// <root>/data/studio-home/vivy-console (Studio's own scratch). The
// production Journal (data/vivy.db, data/demo/, data/workspaces/) is never
// touched (ST-2).
//
// Routes registered on the Studio webServer:
//   GET  /vivy-config.json            -> {"controlPlaneUrl":"http://127.0.0.1:<port>"}
//   GET  /vivy-console/hook.js        -> the frontend-debug bridge script
//   *    /vivy-console/api/*          -> JSON API (gateway + frontend + logs)
//   *    /vivy-web/*                  -> same-origin proxy of the VIVY WEB UI
//                                        (HTML rewritten: asset paths + hook)

import { spawn, spawnSync } from "node:child_process"
import {
  closeSync,
  existsSync,
  mkdirSync,
  openSync,
  readFileSync,
  readSync,
  statSync,
  writeFileSync,
} from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import net from "node:net"
import http from "node:http"

export const name = "vivy-console"
export const inject = ["webServer"]

const here = dirname(fileURLToPath(import.meta.url))
const hookJs = readFileSync(join(here, "hook.js"), "utf8")

// Workspace root: the Studio server is launched from the repository root
// (launch-vivy-studio.ps1 and boot-studio.cmd both `cd` there), so cwd is
// the canonical root. VIVY_INSTALL_DIR / an explicit path may override the
// discovered vivy.exe.
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
const webPrefix = "/vivy-web"
const FE_PORT = 3015

let exeOverride = ""
let resolvedExe = ""
let currentAddr = "127.0.0.1:8787"
let child = null // ChildProcess of the managed gateway, null once exited
let startedAtMs = 0
let feChild = null // ChildProcess of the Vite dev server, null once exited
let feStartedAtMs = 0
let studioPort = 0

function resolveExePath() {
  const candidates = []
  if (exeOverride) candidates.push(exeOverride)
  if (process.env.VIVY_INSTALL_DIR) {
    candidates.push(join(process.env.VIVY_INSTALL_DIR, "vivy.exe"))
  }
  candidates.push(join(root, "vivy.exe"))
  for (const candidate of candidates) {
    if (candidate && existsSync(candidate)) {
      resolvedExe = norm(candidate)
      return resolvedExe
    }
  }
  return ""
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

async function prepare(port) {
  mkdirSync(join(consoleDir, "data", "workspaces"), { recursive: true })
  mkdirSync(join(consoleDir, "data", "skills"), { recursive: true })
  const d = norm(consoleDir)
  const cfg = [
    "server:",
    `  addr: "127.0.0.1:${port}"`,
    "  allowed_origins:",
    `    - "http://127.0.0.1:${studioPort}"`,
    `    - "http://localhost:${studioPort}"`,
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

function tailFile(path, maxBytes) {
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

function readLogs() {
  // Unified timeline: backend (gateway) lines + frontend (Vite dev) lines,
  // each tagged with its source so the client can render one feed with
  // source chips and filter by source.
  const gwOut = tailFile(outLogPath, 512 * 1024)
  const gwErr = tailFile(errLogPath, 256 * 1024)
  const feOut = tailFile(feOutLogPath, 512 * 1024)
  const feErr = tailFile(feErrLogPath, 256 * 1024)
  const gwLines = [...gwOut, ...gwErr.map((line) => (line ? "[err] " + line : line))]
  const feLines = [...feOut, ...feErr.map((line) => (line ? "[err] " + line : line))]
  const lines = [
    ...gwLines.slice(-300).map((line) => ({ src: "backend", text: line })),
    ...feLines.slice(-300).map((line) => ({ src: "frontend", text: line })),
  ]
  return {
    lines,
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

async function startGateway() {
  const exe = resolveExePath()
  if (!exe) {
    return { ok: false, message: "未找到 vivy.exe（可设置 VIVY_INSTALL_DIR 或在面板指定路径）" }
  }
  if (child !== null && child.exitCode === null) {
    return { ok: false, message: "网关已在运行（本控制台管理）" }
  }
  const external = findExternalVivy()
  if (external && external.pid !== (child && child.pid)) {
    return { ok: false, message: `检测到外部 vivy 进程 (PID ${external.pid})，请先停止` }
  }
  const port = await probePort()
  await prepare(port)
  currentAddr = `127.0.0.1:${port}`
  const outFd = openSync(outLogPath, "a")
  const errFd = openSync(errLogPath, "a")
  try {
    child = spawn(exe, [], {
      cwd: dirname(exe) || root,
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
    message: `已启动 (PID ${child.pid})，监听 ${currentAddr}（mock 模式，数据隔离）`,
  }
}

async function stopGateway() {
  if (child !== null && child.exitCode === null) {
    const pid = child.pid
    child.kill()
    child = null
    return { ok: true, message: `已停止 (PID ${pid})` }
  }
  const external = findExternalVivy()
  if (!external) return { ok: false, message: "网关未在运行" }
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
  const exe = resolvedExe || resolveExePath()
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
    exe,
    configPath: norm(cfgPath),
    logPath: norm(outLogPath),
    isolated: true,
    studioPort,
    webPath: webPrefix + "/",
    studioOrigin: `http://127.0.0.1:${studioPort}`,
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
      env: process.env,
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
    message: `前端 dev server 已启动 (PID ${feChild.pid})，监听 ${FE_PORT}（Vite，代理 /rpc → 网关）`,
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

// ---- VIVY WEB same-origin facade ----

function gatewayOrigin() {
  if (child !== null && child.exitCode === null) return `http://${currentAddr}`
  return ""
}

function rewriteHtml(html) {
  const hooked = html.replace("</head>", `<script src="/vivy-console/hook.js"></script></head>`)
  // Rewrite absolute root asset references to the proxied prefix so the UI
  // keeps loading through the Studio origin (same-origin with the console
  // tab). Root paths we own (/vivy-console/*, /vivy-config.json, /rpc) are
  // left untouched.
  return hooked.replace(
    /(\b(?:src|href)=")\/(?!vivy-console\/|vivy-config\.json|rpc(?:\/|$|\?))/g,
    "$1/vivy-web/",
  )
}

function proxyWebRequest(req, res, targetOrigin) {
  const target = new URL(targetOrigin)
  const incoming = new URL(req.url || "/", "http://dsh.internal")
  const pathname = incoming.pathname === webPrefix ? "/" : incoming.pathname.slice(webPrefix.length)
  const options = {
    hostname: target.hostname,
    port: target.port || (target.protocol === "https:" ? 443 : 80),
    path: (pathname || "/") + (incoming.search || ""),
    method: req.method || "GET",
    headers: {
      ...req.headers,
      host: target.host,
      // Identity encoding: the HTML rewrite must see the raw body.
      "accept-encoding": "identity",
    },
  }
  const proxy = http.request(options, (upstream) => {
    const type = String(upstream.headers["content-type"] || "").toLowerCase()
    if (type.includes("text/html")) {
      const chunks = []
      upstream.on("data", (chunk) => chunks.push(chunk))
      upstream.on("end", () => {
        const html = Buffer.concat(chunks).toString("utf8")
        const body = rewriteHtml(html)
        const headers = { ...upstream.headers }
        delete headers["content-length"]
        delete headers["transfer-encoding"]
        headers["content-length"] = Buffer.byteLength(body)
        headers["content-type"] = "text/html; charset=utf-8"
        res.writeHead(upstream.statusCode || 200, headers)
        res.end(body)
      })
      upstream.on("error", () => {
        if (!res.headersSent) res.writeHead(502)
        res.end()
      })
      return
    }
    res.writeHead(upstream.statusCode || 200, upstream.headers)
    upstream.pipe(res)
  })
  proxy.on("error", () => {
    if (!res.headersSent) res.writeHead(502)
    res.end("vivy-web proxy: target unreachable")
  })
  req.pipe(proxy)
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

function text(res, payload, type = "text/javascript; charset=utf-8") {
  res.writeHead(200, { "content-type": type, "cache-control": "no-store" })
  res.end(payload)
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
        exe: resolveExePath(),
        configPath: norm(cfgPath),
        logPath: norm(outLogPath),
        root: norm(root),
      }
      break
    case "POST /start":
      payload = await startGateway()
      break
    case "POST /stop":
      payload = await stopGateway()
      break
    case "POST /restart":
      await stopGateway()
      payload = await startGateway()
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
    if (url.pathname === "/vivy-console/hook.js") {
      text(res, hookJs)
      return
    }
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

async function handleWebProxy(req, res) {
  const origin = gatewayOrigin()
  if (origin === "") {
    res.writeHead(503, { "content-type": "application/json" })
    res.end(JSON.stringify({ ok: false, message: "网关未运行，请先在「网关」页启动" }))
    return
  }
  proxyWebRequest(req, res, origin)
}

function handleVivyConfig(req, res) {
  json(res, { controlPlaneUrl: gatewayOrigin() })
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
        kind: "exact",
        path: "/vivy-config.json",
        handler: handleVivyConfig,
      }),
    "vivy-console: /vivy-config.json (VIVY WEB RPC origin)",
  )
  ctx.effect(
    () =>
      ctx.webServer.register({
        kind: "prefix",
        path: "/vivy-console",
        handler: handleConsoleRoute,
      }),
    "vivy-console: hook + gateway/frontend JSON API",
  )
  ctx.effect(
    () =>
      ctx.webServer.register({
        kind: "prefix",
        path: webPrefix,
        handler: handleWebProxy,
      }),
    "vivy-console: same-origin VIVY WEB facade",
  )
  ctx.effect(
    () => dispose,
    "vivy-console: stop gateway + frontend dev child processes",
  )
}
