// Vivy Studio console — Client half (installed package bundle entry).
//
// Registers a "Vivy 控制台" tab in the conversation view ring
// (`conversation.view` slot, beside Chat / Trajectory / Context) and renders
// the console: gateway lifecycle (status/logs/start/stop/restart) and a
// same-origin VIVY WEB iframe with a frontend-debug bridge (console + RPC
// capture, evaluate).
//
// Hand-authored module in the client-modules handoff format
// (`window.__ModuleLoader__.load({id, factory})`); the injected `require`
// resolves `react` from the browser module table. No build step.
(() => {
  window.__ModuleLoader__.load({
    id: "dsh-vivy-console",
    factory: (require) => {
      var module = { exports: {} }
      const React = require("react")
      const h = React.createElement
      const { useState, useEffect, useRef } = React

      const NS = "dsh-vivy-console"
      const API = "/vivy-console/api"

      // ---- tiny helpers ----
      function call(method, path, payload) {
        return fetch(API + path, {
          method: method,
          headers: payload ? { "content-type": "application/json" } : undefined,
          body: payload ? JSON.stringify(payload) : undefined,
          cache: "no-store",
        })
          .then((r) => r.json())
          .catch((err) => ({ ok: false, message: String((err && err.message) || err) }))
      }
      function fmtTime(ms) {
        const d = new Date(ms)
        if (isNaN(d.getTime())) return "—"
        return d.toLocaleTimeString("en-GB", { hour12: false })
      }
      function trunc(value, n) {
        const text = String(value ?? "")
        return text.length > n ? text.slice(0, n) + "…" : text
      }
      function rpcSummary(body) {
        try {
          const parsed = JSON.parse(body)
          const id = parsed && parsed.id !== undefined ? String(parsed.id) : ""
          const method = parsed && typeof parsed.method === "string" ? parsed.method : ""
          const err = parsed && parsed.error ? (parsed.error.message || "error") : null
          return { id, method, err }
        } catch (e) {
          return { id: "", method: "", err: null }
        }
      }

      // ---- styles ----
      if (!document.getElementById("dsh-vivy-console-style")) {
        const style = document.createElement("style")
        style.id = "dsh-vivy-console-style"
        style.textContent = [
          ".vc-root{box-sizing:border-box;height:100%;display:flex;flex-direction:column;",
          "color:var(--dsw-alias-label-primary);font-size:13px;padding:12px 16px;overflow:hidden}",
          ".vc-tabs{display:flex;gap:6px;flex:none;margin-bottom:10px;border-bottom:1px solid var(--dsw-alias-border-l1);padding-bottom:8px}",
          ".vc-tab{background:transparent;border:1px solid transparent;border-radius:8px;padding:4px 12px;",
          "color:var(--dsw-alias-label-secondary);cursor:pointer;font-size:13px;white-space:nowrap}",
          ".vc-tab.on{background:var(--dsw-alias-bg-layer-1);border-color:var(--dsw-alias-border-l2);color:var(--dsw-alias-label-primary)}",
          ".vc-pane{flex:1;min-height:0;overflow:auto;display:flex;flex-direction:column;gap:8px}",
          ".vc-card{background:var(--dsw-alias-bg-layer-1);border:1px solid var(--dsw-alias-border-l1);border-radius:10px;padding:10px 12px}",
          ".vc-row{display:flex;gap:6px;padding:2px 0;font-size:12px}",
          ".vc-key{color:var(--dsw-alias-label-secondary);min-width:84px;flex:none}",
          ".vc-val{word-break:break-all}",
          ".vc-btn{height:28px;padding:0 12px;border:1px solid var(--dsw-alias-border-l2);border-radius:8px;",
          "background:var(--dsw-alias-bg-layer-1);color:var(--dsw-alias-label-primary);cursor:pointer;font-size:12px;white-space:nowrap}",
          ".vc-btn:disabled{opacity:.45;cursor:default}",
          ".vc-btn.primary{border-color:var(--dsw-alias-brand-primary);color:var(--dsw-alias-brand-primary)}",
          ".vc-btn.danger{border-color:var(--dsw-alias-state-error-primary);color:var(--dsw-alias-state-error-primary)}",
          ".vc-msg{min-height:16px;color:var(--dsw-alias-label-secondary);font-size:12px;white-space:pre-wrap;word-break:break-all}",
          ".vc-input{height:28px;padding:0 8px;background:var(--dsw-alias-bg-base);border:1px solid var(--dsw-alias-border-l1);",
          "border-radius:6px;color:var(--dsw-alias-label-primary);font-size:12px;box-sizing:border-box}",
          ".vc-pre{flex:1;min-height:120px;max-height:40vh;overflow:auto;margin:0;padding:8px 10px;",
          "background:var(--dsw-alias-bg-base);border:1px solid var(--dsw-alias-border-l1);border-radius:8px;",
          "font-family:ui-monospace,Consolas,monospace;font-size:11px;line-height:1.5;white-space:pre-wrap;word-break:break-all}",
          ".vc-web{flex:1;min-height:0;display:flex;flex-direction:column;gap:8px}",
          ".vc-frame{flex:1;min-height:0;border:1px solid var(--dsw-alias-border-l1);border-radius:10px;background:#fff}",
          ".vc-capture{flex:1;min-height:0;overflow:auto;border:1px solid var(--dsw-alias-border-l1);border-radius:8px;",
          "background:var(--dsw-alias-bg-base);font-family:ui-monospace,Consolas,monospace;font-size:11px;line-height:1.5}",
          ".vc-crow{padding:3px 8px;border-bottom:1px solid var(--dsw-alias-border-l1);white-space:pre-wrap;word-break:break-all}",
          ".vc-crow b{font-weight:600}",
          ".vc-crow.err{color:var(--dsw-alias-state-error-primary)}",
          ".vc-crow.warn{color:var(--dsw-alias-state-warn-primary)}",
          ".vc-crow.ok{color:var(--dsw-alias-state-success-primary)}",
          ".vc-chip{background:none;border:1px solid var(--dsw-alias-border-l2);border-radius:999px;padding:1px 10px;",
          "color:var(--dsw-alias-label-secondary);cursor:pointer;font-size:11px}",
          ".vc-chip.on{color:var(--dsw-alias-brand-primary);border-color:var(--dsw-alias-brand-primary)}",
          ".vc-empty{color:var(--dsw-alias-label-tertiary);font-size:12px;padding:8px 2px}",
        ].join("")
        document.head.appendChild(style)
      }

      // ---- Gateway pane ----
      function GatewayPane({ status }) {
        const [busy, setBusy] = useState("")
        const [msg, setMsg] = useState("")
        const [exe, setExe] = useState("")
        useEffect(() => {
          if (!exe && status && status.exe) setExe(status.exe)
        }, [status && status.exe]) // eslint-disable-line react-hooks/exhaustive-deps
        function act(name, label) {
          setBusy(label)
          setMsg("")
          call("POST", "/" + name).then((r) => {
            setMsg((r && r.message) || label + " 完成")
            setBusy("")
          })
        }
        function applyExe() {
          call("POST", "/setExe", { path: exe }).then((r) => {
            setMsg((r && r.ok && "已应用 EXE 路径") || "设置失败")
          })
        }
        const running = !!(status && status.running)
        const listening = !!(status && status.listening)
        const stateText = running
          ? "运行中 " + (status.managed ? "(本控制台管理)" : "(外部进程)")
          : listening
            ? "已停止（端口仍被监听）"
            : "已停止"
        return h("div", { className: "vc-pane" }, [
          h("div", { className: "vc-card" }, [
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "状态"), h("span", { className: "vc-val" }, stateText)]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "PID"), h("span", { className: "vc-val" }, String((status && status.pid) || "—"))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "监听"), h("span", { className: "vc-val" }, String((status && status.addr) || "—"))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "启动于"), h("span", { className: "vc-val" }, status && status.startedAtMs ? fmtTime(status.startedAtMs) : "—")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "EXE"), h("span", { className: "vc-val" }, trunc((status && status.exe) || "…", 120))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "配置"), h("span", { className: "vc-val" }, trunc((status && status.configPath) || "…", 120))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "数据隔离"), h("span", { className: "vc-val" }, "data/studio-home/vivy-console，不触碰生产 Journal")]),
          ]),
          h("div", { className: "vc-card" }, [
            h("div", { style: { display: "flex", gap: 8 } }, [
              h("button", { className: "vc-btn primary", disabled: !!busy || running, onClick: () => act("start", "启动") }, busy === "启动" ? "启动中…" : "▶ 启动"),
              h("button", { className: "vc-btn", disabled: !!busy || !running, onClick: () => act("stop", "停止") }, busy === "停止" ? "停止中…" : "■ 停止"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("restart", "重启") }, busy === "重启" ? "重启中…" : "⟳ 重启"),
            ]),
            h("div", { style: { display: "flex", gap: 8, marginTop: 8 } }, [
              h("input", { className: "vc-input", style: { flex: 1 }, value: exe, placeholder: "vivy.exe 路径（留空自动解析）", onChange: (e) => setExe(e.target.value) }),
              h("button", { className: "vc-btn", onClick: applyExe }, "应用 EXE"),
            ]),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, msg),
          ]),
        ])
      }

      // ---- VIVY WEB pane ----
      function WebPane({ status }) {
        const running = !!(status && status.running)
        const [mode, setMode] = useState("proxy")
        const [frameKey, setFrameKey] = useState(0)
        const [events, setEvents] = useState([])
        const [filter, setFilter] = useState("all")
        const [expr, setExpr] = useState("")
        const [result, setResult] = useState("")
        const iframeRef = useRef(null)
        const seqRef = useRef(0)
        const lastAddrRef = useRef("")

        const src = mode === "proxy" ? "/vivy-web/" : "http://127.0.0.1:3015/"

        useEffect(() => {
          function onMessage(ev) {
            if (iframeRef.current && ev.source !== iframeRef.current.contentWindow) return
            const d = ev.data
            if (!d || d.__vivyConsole !== true) return
            if (d.type === "ready" || d.type === "pong" || d.type === "ws") return
            if (d.type === "eval-result") {
              setResult(d.value === undefined ? "undefined" : d.value)
              return
            }
            setEvents((prev) => [...prev.slice(-299), d])
          }
          window.addEventListener("message", onMessage)
          return () => window.removeEventListener("message", onMessage)
        }, [])

        // Reload the proxied iframe when the gateway addr changes.
        useEffect(() => {
          if (mode !== "proxy") return
          const addr = status && status.addr
          if (addr && addr !== lastAddrRef.current) {
            lastAddrRef.current = addr
            setFrameKey((k) => k + 1)
          }
        }, [status, mode])

        function evaluate() {
          const win = iframeRef.current && iframeRef.current.contentWindow
          if (!win || !expr.trim()) return
          const id = "ev" + String(++seqRef.current)
          setResult("…")
          win.postMessage({ __vivyConsole: "cmd", cmd: "evaluate", expr: expr, id: id }, "*")
        }

        const filtered = events.filter((e) => {
          if (filter === "all") return true
          if (filter === "console") return e.type === "console"
          if (filter === "rpc") return e.type === "wsmsg"
          if (filter === "err") return e.type === "pageerror" || e.type === "unhandledrejection" || e.type === "console" && e.payload && e.payload.level === "error"
          return true
        })

        return h("div", { className: "vc-web" }, [
          h("div", { style: { display: "flex", gap: 8, alignItems: "center", flex: "none" } }, [
            h("button", { className: "vc-btn", onClick: () => setFrameKey((k) => k + 1) }, "⟳ 刷新"),
            h("button", {
              className: "vc-btn",
              onClick: () => window.open(src, "_blank"),
            }, "新标签打开"),
            h("span", { className: "vc-key" }, "目标"),
            h("select", { className: "vc-input", value: mode, onChange: (e) => setMode(e.target.value) }, [
              h("option", { value: "proxy" }, "内嵌（网关 UI，同源调试）"),
              h("option", { value: "dev" }, "开发（Vite 3015，直接显示）"),
            ]),
            h("button", { className: "vc-chip" + (filter === "all" ? " on" : ""), onClick: () => setFilter("all") }, "全部"),
            h("button", { className: "vc-chip" + (filter === "console" ? " on" : ""), onClick: () => setFilter("console") }, "控制台"),
            h("button", { className: "vc-chip" + (filter === "rpc" ? " on" : ""), onClick: () => setFilter("rpc") }, "RPC"),
            h("button", { className: "vc-chip" + (filter === "err" ? " on" : ""), onClick: () => setFilter("err") }, "错误"),
            h("button", { className: "vc-btn", onClick: () => setEvents([]) }, "清空"),
          ]),
          !running && mode === "proxy"
            ? h("div", { className: "vc-card", style: { flex: 1, display: "flex", alignItems: "center", justifyContent: "center", color: "var(--dsw-alias-label-secondary)" } }, "网关未运行 — 请先在「网关」页启动，再回到这里查看 VIVY WEB。")
            : h("iframe", {
                ref: iframeRef,
                key: String(frameKey) + ":" + mode,
                className: "vc-frame",
                src: src,
                title: "VIVY WEB",
              }),
          h("div", { className: "vc-capture" }, filtered.length === 0
            ? h("div", { className: "vc-empty" }, mode === "proxy" ? "等待捕获（VIVY WEB 的控制台与 RPC 流量会显示在这里）…" : "开发（直连）模式下无法注入调试桥，无捕获。")
            : filtered.map((e, i) => {
                if (e.type === "console") {
                  return h("div", { className: "vc-crow" + (e.payload && e.payload.level === "error" ? " err" : e.payload && e.payload.level === "warn" ? " warn" : ""), key: i }, [
                    h("b", null, fmtTime(e.t) + " [console] " + (e.payload ? e.payload.level : "")),
                    " " + (e.payload && e.payload.args ? e.payload.args.join(" ") : ""),
                  ])
                }
                if (e.type === "wsmsg") {
                  const body = (e.payload && e.payload.body) || ""
                  const sum = rpcSummary(body)
                  const cls = e.payload && e.payload.dir === "send" ? "" : sum.err ? " err" : " ok"
                  return h("div", { className: "vc-crow" + cls, key: i }, [
                    h("b", null, fmtTime(e.t) + " [rpc] " + (e.payload && e.payload.dir === "send" ? "→ " : "← ")),
                    sum.method || ("#" + sum.id),
                    "  " + trunc(body, 160),
                  ])
                }
                return h("div", { className: "vc-crow err", key: i }, [
                  h("b", null, fmtTime(e.t) + " [" + e.type + "] "),
                  (e.payload && (e.payload.message || e.payload.level || "")) || "",
                ])
              })),
          h("div", { style: { display: "flex", gap: 8, alignItems: "center", flex: "none" } }, [
            h("input", {
              className: "vc-input",
              style: { flex: 1 },
              value: expr,
              placeholder: "在 VIVY WEB 页面中求值表达式，如 document.title",
              onChange: (e) => setExpr(e.target.value),
              onKeyDown: (e) => { if (e.key === "Enter") evaluate() },
            }),
            h("button", { className: "vc-btn primary", onClick: evaluate, disabled: mode !== "proxy" }, "求值"),
          ]),
          result ? h("div", { className: "vc-msg" }, "= " + trunc(result, 400)) : null,
        ])
      }

      // ---- Logs pane ----
      function LogsPane() {
        const [lines, setLines] = useState([])
        const [paused, setPaused] = useState(false)
        const [msg, setMsg] = useState("")
        const preRef = useRef(null)
        useEffect(() => {
          if (paused) return
          let alive = true
          const tick = () =>
            call("GET", "/logs").then((l) => {
              if (!alive) return
              if (l && Array.isArray(l.lines)) setLines(l.lines)
              else setMsg((l && l.message) || "日志读取失败")
            }).catch(() => {})
          tick()
          const timer = setInterval(tick, 2000)
          return () => { alive = false; clearInterval(timer) }
        }, [paused])
        useEffect(() => {
          if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight
        }, [lines])
        return h("div", { className: "vc-pane" }, [
          h("div", { style: { display: "flex", gap: 8, alignItems: "center", flex: "none" } }, [
            h("button", { className: "vc-btn", onClick: () => setPaused(!paused) }, paused ? "▶ 继续" : "❚❚ 暂停"),
            h("span", { className: "vc-key" }, "尾随网关 stdout/stderr（最多 300 行）"),
          ]),
          h("pre", { ref: preRef, className: "vc-pre" }, lines.length ? lines.join("\n") : "（暂无日志）"),
          msg ? h("div", { className: "vc-msg" }, msg) : null,
        ])
      }

      // ---- Root view ----
      const SECTIONS = [
        ["gateway", "网关"],
        ["web", "VIVY WEB"],
        ["logs", "日志"],
      ]

      function VivyConsoleView() {
        const [section, setSection] = useState("gateway")
        const [status, setStatus] = useState(null)
        useEffect(() => {
          let alive = true
          const tick = () =>
            call("GET", "/status").then((s) => {
              if (alive && s && typeof s.running === "boolean") setStatus(s)
            }).catch(() => {})
          tick()
          const timer = setInterval(tick, 2000)
          return () => { alive = false; clearInterval(timer) }
        }, [])
        return h("div", { className: "vc-root" }, [
          h("div", { className: "vc-tabs" }, SECTIONS.map(([id, label]) =>
            h("button", { className: "vc-tab" + (id === section ? " on" : ""), key: id, onClick: () => setSection(id) }, label),
          )),
          section === "gateway" ? h(GatewayPane, { status }) : null,
          section === "web" ? h(WebPane, { status }) : null,
          section === "logs" ? h(LogsPane, null) : null,
        ])
      }

      // ---- registration ----
      function apply(ctx) {
        ctx.slots.inject("conversation.view", () =>
          ctx.slots.register(
            {
              name: "conversation.view",
              id: "vivy-console",
              order: 30,
              label: () => "Vivy 控制台",
            },
            (props) => h(VivyConsoleView, props),
          ),
        )
      }

      module.exports = { name: NS, inject: ["slots"], apply }
      return module.exports
    },
  })
})()
