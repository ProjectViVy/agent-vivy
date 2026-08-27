// Vivy Studio console — Client half (installed package bundle entry).
//
// Registers a "Vivy 控制台" tab in the conversation view ring
// (`conversation.view` slot, beside Chat / Trajectory / Context) and renders
// the unified development loop: the pure-API backend (vivy_headless, no
// embedded frontend), the DEV dev server (pnpm dev in ui/, :3015) as the
// single frontend, and one unified log timeline for both processes.
//
// There is intentionally no VIVY WEB debug pane: the only app is the dev
// server at http://127.0.0.1:3015 — open it in the normal browser.
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
          ".vc-chip{background:none;border:1px solid var(--dsw-alias-border-l2);border-radius:999px;padding:1px 10px;",
          "color:var(--dsw-alias-label-secondary);cursor:pointer;font-size:11px}",
          ".vc-chip.on{color:var(--dsw-alias-brand-primary);border-color:var(--dsw-alias-brand-primary)}",
        ].join("")
        document.head.appendChild(style)
      }

      // ---- Backend pane (pure API, no embedded frontend) ----
      function BackendPane({ status }) {
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
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "监听"), h("span", { className: "vc-val" }, String((status && status.addr) || "—") + "（/rpc 控制面）")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "启动于"), h("span", { className: "vc-val" }, status && status.startedAtMs ? fmtTime(status.startedAtMs) : "—")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "EXE"), h("span", { className: "vc-val" }, trunc((status && status.exe) || "…", 120))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "形态"), h("span", { className: "vc-val" }, "纯 API 后端（vivy_headless 编译，无内嵌前端）")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "编译"), h("span", { className: "vc-val" }, (status && status.autoBuild) === false ? "使用指定 EXE，不自动编译" : "go build -tags vivy_headless（每次启动增量编译）")]),
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
              h("input", { className: "vc-input", style: { flex: 1 }, value: exe, placeholder: "后端 EXE 路径（留空自动编译 vivy_headless 后端）", onChange: (e) => setExe(e.target.value) }),
              h("button", { className: "vc-btn", onClick: applyExe }, "应用 EXE"),
            ]),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, msg),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, "启动后，前端在「前端」页启动 pnpm dev（:3015），其 /rpc 代理指向本后端。"),
          ]),
        ])
      }

      // ---- Frontend dev server pane (the one DEV frontend) ----
      function FrontendPane({ status }) {
        const [busy, setBusy] = useState("")
        const [msg, setMsg] = useState("")
        const fe = (status && status.frontend) || {}
        function act(name, label) {
          setBusy(label)
          setMsg("")
          call("POST", "/frontend/" + name).then((r) => {
            setMsg((r && r.message) || label + " 完成")
            setBusy("")
          })
        }
        const running = !!(fe && fe.running)
        const listening = !!(fe && fe.listening)
        const stateText = running
          ? "运行中 " + (fe.managed ? "(本控制台管理)" : "(外部进程)")
          : listening
            ? "已停止（端口仍被监听）"
            : fe.dirReady
              ? "已停止"
              : "未就绪（缺少 ui/package.json）"
        return h("div", { className: "vc-pane" }, [
          h("div", { className: "vc-card" }, [
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "状态"), h("span", { className: "vc-val" }, stateText)]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "PID"), h("span", { className: "vc-val" }, String((fe && fe.pid) || "—"))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "入口"), h("span", { className: "vc-val" }, String((fe && fe.addr) || "—") + "（Vite DEV 开发服务器，唯一前端入口）")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "启动于"), h("span", { className: "vc-val" }, fe && fe.startedAtMs ? fmtTime(fe.startedAtMs) : "—")]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "命令"), h("span", { className: "vc-val" }, String((fe && fe.command) || "—") + " @ " + trunc((fe && fe.cwd) || "…", 120))]),
            h("div", { className: "vc-row" }, [h("span", { className: "vc-key" }, "代理"), h("span", { className: "vc-val" }, "/rpc → " + String((fe && fe.rpcTarget) || "…") + "（Vite 代理到纯 API 后端）")]),
          ]),
          h("div", { className: "vc-card" }, [
            h("div", { style: { display: "flex", gap: 8 } }, [
              h("button", { className: "vc-btn primary", disabled: !!busy || running, onClick: () => act("start", "启动") }, busy === "启动" ? "启动中…" : "▶ 启动"),
              h("button", { className: "vc-btn", disabled: !!busy || !running, onClick: () => act("stop", "停止") }, busy === "停止" ? "停止中…" : "■ 停止"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("restart", "重启") }, busy === "重启" ? "重启中…" : "⟳ 重启"),
              h("button", { className: "vc-btn", disabled: !(fe && fe.url), onClick: () => window.open(fe.url, "vivy-dev") }, "打开 " + String((fe && fe.addr) || "127.0.0.1:3015")),
            ]),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, msg),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, "前端 dev 日志与后端日志在「日志」页统一时间线显示。"),
          ]),
        ])
      }

      // ---- Logs pane (unified timeline) ----
      function LogsPane() {
        const [lines, setLines] = useState([])
        const [paused, setPaused] = useState(false)
        const [srcFilter, setSrcFilter] = useState("all")
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
        const visible = lines.filter((l) => srcFilter === "all" || (l && l.src === srcFilter))
        return h("div", { className: "vc-pane" }, [
          h("div", { style: { display: "flex", gap: 8, alignItems: "center", flex: "none", flexWrap: "wrap" } }, [
            h("button", { className: "vc-btn", onClick: () => setPaused(!paused) }, paused ? "▶ 继续" : "❚❚ 暂停"),
            h("button", { className: "vc-btn", onClick: () => setLines([]) }, "清空"),
            h("span", { className: "vc-key" }, "来源"),
            h("button", { className: "vc-chip" + (srcFilter === "all" ? " on" : ""), onClick: () => setSrcFilter("all") }, "全部"),
            h("button", { className: "vc-chip" + (srcFilter === "backend" ? " on" : ""), onClick: () => setSrcFilter("backend") }, "后端"),
            h("button", { className: "vc-chip" + (srcFilter === "frontend" ? " on" : ""), onClick: () => setSrcFilter("frontend") }, "前端"),
            h("span", { className: "vc-key" }, "后端日志尾随 " + (lines.filter((l) => l && l.src === "backend").length) + " 行 / 前端 " + (lines.filter((l) => l && l.src === "frontend").length) + " 行"),
          ]),
          h("pre", { ref: preRef, className: "vc-pre" }, visible.length
            ? visible.map((l) => "[" + (l.src === "frontend" ? "前端" : "后端") + "] " + l.text).join("\n")
            : "（暂无日志）"),
          msg ? h("div", { className: "vc-msg" }, msg) : null,
        ])
      }

      // ---- Root view ----
      const SECTIONS = [
        ["backend", "后端"],
        ["frontend", "前端"],
        ["logs", "日志"],
      ]

      function VivyConsoleView() {
        const [section, setSection] = useState("backend")
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
          section === "backend" ? h(BackendPane, { status }) : null,
          section === "frontend" ? h(FrontendPane, { status }) : null,
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