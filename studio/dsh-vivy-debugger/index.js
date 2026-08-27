// Vivy Studio first-party gateway debugger (Host half).
//
// Runs as a profile bundle plugin inside the Studio server process (real
// Node, no vm sandbox): registers /vivy-debugger/api/* JSON routes on the
// webServer service and manages a vivy.exe gateway child process with
// file-backed logs. The browser half (client.js) is injected into index.html
// through webServer.tapIndex, exactly like the skin plugin does.
//
// Air gap: all gateway data lives under <root>/data/studio-home/vivy-debugger
// (Studio's own scratch). The production Journal (data/vivy.db, data/demo/,
// data/workspaces/) is never touched.

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

export const name = "vivy-debugger"
export const inject = ["webServer"]

const here = dirname(fileURLToPath(import.meta.url))
const clientJs = readFileSync(join(here, "client.js"), "utf8")

// Workspace root: the Studio server is launched from the repository root
// (launch-vivy-studio.ps1 and boot-studio.cmd both `cd` there), so cwd is
// the canonical root. VIVY_INSTALL_DIR / an explicit path may override the
// discovered vivy.exe.
const root = process.cwd().replace(/[\\/]+$/, "")
const debugDir = join(root, "data", "studio-home", "vivy-debugger")
const cfgPath = join(debugDir, "config.yaml")
const outLogPath = join(debugDir, "gateway.out.log")
const errLogPath = join(debugDir, "gateway.err.log")

const norm = (p) => String(p || "").replace(/\\/g, "/")

let exeOverride = ""
let resolvedExe = ""
let currentAddr = "127.0.0.1:8787"
let child = null // ChildProcess of the managed gateway, null once exited
let startedAtMs = 0

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
  mkdirSync(join(debugDir, "data", "workspaces"), { recursive: true })
  mkdirSync(join(debugDir, "data", "skills"), { recursive: true })
  const d = norm(debugDir)
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
    return { ok: false, message: "网关已在运行（本调试器管理）" }
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
  }
}

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

const apiPrefix = "/vivy-debugger/api"

async function handleApi(req, res) {
  try {
    const url = new URL(req.url || "/", "http://x")
    const route = url.pathname.slice(apiPrefix.length) || "/"
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
      default:
        res.writeHead(404, { "content-type": "application/json" })
        res.end(JSON.stringify({ ok: false, message: `no route: ${method} ${route}` }))
        return
    }
    json(res, payload)
  } catch (error) {
    json(res, { ok: false, message: error instanceof Error ? error.message : String(error) })
  }
}

export function apply(ctx) {
  ctx.effect(
    () =>
      ctx.webServer.register({
        kind: "prefix",
        path: apiPrefix,
        handler: handleApi,
      }),
    "vivy-debugger: gateway status/log/control API",
  )
  ctx.effect(
    () =>
      ctx.webServer.tapIndex((html) => {
        const script = `<script data-plugin="dsh-vivy-debugger">${clientJs}</script>`
        const body = /<body(?:\s[^>]*)?>/i.exec(html)
        if (body === null) return html + script
        const at = body.index + body[0].length
        return html.slice(0, at) + script + html.slice(at)
      }),
    "vivy-debugger: floating gateway control UI",
  )
}
