// Vivy Studio first-party console (Host half).
//
// Runs as a profile bundle plugin inside the Studio server process (real
// Node, no vm sandbox). Supersedes dsh-vivy-debugger: it manages the
// vivy.exe gateway child process (status/logs/start/stop/restart/exe
// override), serves a same-origin facade of the VIVY WEB UI at /vivy-web/
// with a frontend-debug bridge injected into the proxied HTML, and drives
// the Studio distribution ledger through vivy-studio.exe
// (pack/eval/release/reject/install/rollback/inspect/list).
//
// Air gap: all gateway data lives under
// <root>/data/studio-home/vivy-console (Studio's own scratch). The
// production Journal (data/vivy.db, data/demo/, data/workspaces/) is never
// touched (ST-2).
//
// Routes registered on the Studio webServer:
//   GET  /vivy-config.json            -> {"controlPlaneUrl":"http://127.0.0.1:<port>"}
//   GET  /vivy-console/hook.js        -> the frontend-debug bridge script
//   *    /vivy-console/api/*          -> JSON API (gateway + lifecycle)
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

const norm = (p) => String(p || "").replace(/\\/g, "/")

const apiPrefix = "/vivy-console/api"
const webPrefix = "/vivy-web"
const MAX_JOB_LINES = 500

let exeOverride = ""
let resolvedExe = ""
let currentAddr = "127.0.0.1:8787"
let child = null // ChildProcess of the managed gateway, null once exited
let startedAtMs = 0
let studioPort = 0

// ---- lifecycle jobs (one concurrent, killed with the plugin) ----
const jobs = new Map()
let jobSeq = 0

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

function resolveStudioExe() {
  const candidates = []
  if (process.env.VIVY_STUDIO) candidates.push(process.env.VIVY_STUDIO)
  candidates.push(join(root, "vivy-studio.exe"))
  for (const candidate of candidates) {
    if (candidate && existsSync(candidate)) return norm(candidate)
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
  const out = tailFile(outLogPath, 512 * 1024)
  const err = tailFile(errLogPath, 256 * 1024)
  const tagged = err.map((line) => (line ? "[err] " + line : line))
  const lines = [...out, ...tagged]
  const tail = lines.slice(-300)
  return {
    lines: tail,
    truncated: lines.length > 300,
    logPath: norm(outLogPath),
    running: child !== null && child.exitCode === null,
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
  }
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

// ---- lifecycle (vivy-studio.exe) ----

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
        studio: resolveStudioExe(),
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
    "vivy-console: hook + gateway/lifecycle JSON API",
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
    "vivy-console: stop gateway and lifecycle jobs",
  )
}
