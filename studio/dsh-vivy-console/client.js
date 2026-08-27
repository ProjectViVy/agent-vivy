// Vivy Studio console — Client half (installed package bundle entry).
//
// Registers a "Vivy 控制台" tab in the conversation view ring
// (`conversation.view` slot, beside Chat / Trajectory / Context) and renders
// the unified development loop as a **总控台** (main console): one-click
// start/stop/restart for both sides, backend status and frontend status on
// one screen so nothing feels split, plus the unified log timeline.
//
// The unified model (one dev server only):
//   * backend = pure-API vivy binary (vivy_headless, no embedded frontend),
//     auto-compiled into Studio scratch on start
//   * frontend = the DEV dev server (pnpm dev in ui/, :3015) — the sole app
// Frontend debugging happens in the normal browser at http://127.0.0.1:3015.
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
      const { useState, useEffect, useRef, useCallback } = React

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
      function row(key, value) {
        return h("div", { className: "vc-row" }, [
          h("span", { className: "vc-key" }, key),
          h("span", { className: "vc-val" }, value),
        ])
      }
      function stateText(running, managed, listening) {
        if (running) return "运行中 " + (managed ? "(本控制台管理)" : "(外部进程)")
        if (listening) return "已停止（端口仍被监听）"
        return "已停止"
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
          ".vc-card-title{font-size:12px;font-weight:600;margin-bottom:6px;color:var(--dsw-alias-label-secondary)}",
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
          ".vc-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:6px}",
          ".vc-field{display:flex;flex-direction:column;gap:2px;font-size:11px;color:var(--dsw-alias-label-secondary)}",
          ".vc-tbl{width:100%;border-collapse:collapse;font-size:12px}",
          ".vc-tbl th,.vc-tbl td{border:1px solid var(--dsw-alias-border-l1);padding:4px 6px;text-align:left;vertical-align:top}",
          ".vc-tbl th{background:var(--dsw-alias-bg-layer-2);color:var(--dsw-alias-label-secondary);font-weight:600;position:sticky;top:0}",
          ".vc-empty{color:var(--dsw-alias-label-tertiary);font-size:12px;padding:8px 2px}",
        ].join("")
        document.head.appendChild(style)
      }

      // ---- 总控台 (main console) ----
      function OverviewPane({ status }) {
        const [busy, setBusy] = useState("")
        const [msg, setMsg] = useState("")
        const [exe, setExe] = useState("")
        useEffect(() => {
          if (!exe && status && status.exe) setExe(status.exe)
        }, [status && status.exe]) // eslint-disable-line react-hooks/exhaustive-deps

        const bk = !!(status && status.running)
        const fe = !!(status && status.frontend && status.frontend.running)
        const allRunning = bk && fe
        const overall = allRunning
          ? "全部运行中"
          : bk
            ? "后端运行中 · 前端未运行"
            : fe
              ? "前端运行中 · 后端未运行"
              : "未运行"

        function runAction(label, fn) {
          setBusy(label)
          setMsg("")
          Promise.resolve(fn()).then((text) => {
            setMsg(text || label + " 完成")
            setBusy("")
          })
        }
        function act(prefix, name, label) {
          runAction(label, () =>
            call("POST", "/" + prefix + name).then((r) => (r && r.message) || label + " 完成"),
          )
        }

        function startAll() {
          runAction("一键启动", async () => {
            const b = await call("POST", "/start")
            if (b && b.ok) {
              const f = await call("POST", "/frontend/start")
              return f && f.ok ? "一键启动完成：后端 + 前端均已启动" : "后端已启动；前端启动失败：" + ((f && f.message) || "")
            }
            if (b && !b.ok && (b.message || "").indexOf("已在运行") >= 0) {
              const f = await call("POST", "/frontend/start")
              return f && f.ok ? "后端已在运行；前端已启动" : "后端已在运行；前端启动失败：" + ((f && f.message) || "")
            }
            return "一键启动失败（后端）：" + ((b && b.message) || "")
          })
        }
        function stopAll() {
          runAction("一键停止", async () => {
            const f = await call("POST", "/frontend/stop")
            const b = await call("POST", "/stop")
            return [
              f && f.ok ? "前端已停止" : (f && f.message) || "前端停止失败",
              b && b.ok ? "后端已停止" : (b && b.message) || "后端停止失败",
            ].join("；")
          })
        }
        function restartAll() {
          runAction("一键重启", async () => {
            await call("POST", "/stop")
            await call("POST", "/frontend/stop")
            const b = await call("POST", "/start")
            if (!(b && b.ok)) return "一键重启失败（后端）：" + ((b && b.message) || "")
            const f = await call("POST", "/frontend/start")
            return f && f.ok
              ? "一键重启完成：后端 + 前端均重新启动"
              : "后端已重启；前端启动失败：" + ((f && f.message) || "")
          })
        }
        function applyExe() {
          call("POST", "/setExe", { path: exe }).then((r) => {
            setMsg((r && r.ok && "已应用 EXE 路径") || "设置失败")
          })
        }

        const feStatus = (status && status.frontend) || {}
        const cards = [
          h("div", { className: "vc-card", style: { flex: "1 1 260px", minWidth: 260 } }, [
            h("div", { className: "vc-card-title" }, "后端状态"),
            row("状态", stateText(bk, !!(status && status.managed), !!(status && status.listening))),
            row("PID", String((status && status.pid) || "—")),
            row("监听", String((status && status.addr) || "—") + "（/rpc 控制面）"),
            row("启动于", status && status.startedAtMs ? fmtTime(status.startedAtMs) : "—"),
            row("EXE", trunc((status && status.exe) || "…", 100)),
            row("形态", "纯 API（vivy_headless，无内嵌前端）"),
            row("编译", (status && status.autoBuild) === false ? "指定 EXE，不自动编译" : "go build -tags vivy_headless（每次启动增量编译）"),
            h("div", { style: { display: "flex", gap: 8, marginTop: 8 } }, [
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("", "start", "后端启动") }, busy === "后端启动" ? "启动中…" : "▶ 启动"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("", "stop", "后端停止") }, busy === "后端停止" ? "停止中…" : "■ 停止"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("", "restart", "后端重启") }, busy === "后端重启" ? "重启中…" : "⟳ 重启"),
            ]),
            h("div", { style: { display: "flex", gap: 8, marginTop: 8 } }, [
              h("input", { className: "vc-input", style: { flex: 1 }, value: exe, placeholder: "后端 EXE 路径（留空自动编译 vivy_headless）", onChange: (e) => setExe(e.target.value) }),
              h("button", { className: "vc-btn", onClick: applyExe }, "应用 EXE"),
            ]),
          ]),
          h("div", { className: "vc-card", style: { flex: "1 1 260px", minWidth: 260 } }, [
            h("div", { className: "vc-card-title" }, "前端状态"),
            row("状态", stateText(fe, !!(feStatus && feStatus.managed), !!(feStatus && feStatus.listening))),
            row("PID", String((feStatus && feStatus.pid) || "—")),
            row("入口", String((feStatus && feStatus.addr) || "127.0.0.1:3015") + "（Vite DEV 开发服务器，唯一前端）"),
            row("启动于", feStatus && feStatus.startedAtMs ? fmtTime(feStatus.startedAtMs) : "—"),
            row("命令", String((feStatus && feStatus.command) || "pnpm dev") + " @ " + trunc((feStatus && feStatus.cwd) || "…", 100)),
            row("代理", "/rpc → " + String((feStatus && feStatus.rpcTarget) || "…") + "（Vite 代理到纯 API 后端）"),
            h("div", { style: { display: "flex", gap: 8, marginTop: 8 } }, [
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("frontend/", "start", "前端启动") }, busy === "前端启动" ? "启动中…" : "▶ 启动"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("frontend/", "stop", "前端停止") }, busy === "前端停止" ? "停止中…" : "■ 停止"),
              h("button", { className: "vc-btn", disabled: !!busy, onClick: () => act("frontend/", "restart", "前端重启") }, busy === "前端重启" ? "重启中…" : "⟳ 重启"),
              h("button", { className: "vc-btn", disabled: !(feStatus && feStatus.url), onClick: () => window.open(feStatus.url, "vivy-dev") }, "打开 " + String((feStatus && feStatus.addr) || "127.0.0.1:3015")),
            ]),
          ]),
        ]

        return h("div", { className: "vc-pane" }, [
          h("div", { className: "vc-card" }, [
            h("div", { style: { display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" } }, [
              h("span", { className: "vc-key" }, "总览"),
              h("span", { className: "vc-val", style: { fontWeight: 600 } }, overall),
              h("span", { style: { flex: 1 } }, null),
              h("button", { className: "vc-btn primary", disabled: !!busy || allRunning, onClick: startAll }, busy === "一键启动" ? "启动中…" : "▶ 一键启动"),
              h("button", { className: "vc-btn danger", disabled: !!busy || (!bk && !fe), onClick: stopAll }, busy === "一键停止" ? "停止中…" : "■ 一键停止"),
              h("button", { className: "vc-btn", disabled: !!busy || !bk, onClick: restartAll }, busy === "一键重启" ? "重启中…" : "⟳ 一键重启"),
            ]),
            msg ? h("div", { className: "vc-msg", style: { marginTop: 8 } }, msg) : null,
          ]),
          h("div", { style: { display: "flex", gap: 8, flexWrap: "wrap", alignItems: "flex-start" } }, cards),
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

      // ---- 打包与版本 (packaging & version management) ----
      // Studio distribution, deliberately separate from the dev loop
      // (总控台): this page never starts or stops the backend/frontend. All
      // actions run through vivy-studio.exe on the pinned worktree; release
      // keeps the human gate (NG-25).
      const LEDGER_KINDS = ["generations", "evals", "releases", "installs", "events", "worktrees"]
      const ACTION_LABELS = {
        pack: "打包 pack",
        eval: "评测 eval",
        release: "发布 release",
        reject: "拒绝 reject",
        install: "安装 install",
        rollback: "回滚 rollback",
        inspect: "检查 inspect",
      }

      function PackagePane() {
        const [kind, setKind] = useState("generations")
        const [data, setData] = useState(null)
        const [err, setErr] = useState("")
        const [job, setJob] = useState(null)
        const [form, setForm] = useState({
          with: "", candidate: "", baseline: "", suite: "",
          generation: "", eval: "", release: "", target: "", confirm: false,
        })
        const preRef = useRef(null)

        const refresh = useCallback(() => {
          call("GET", "/lifecycle/list?kind=" + encodeURIComponent(kind)).then((r) => {
            if (r && r.ok) { setData(r.value); setErr("") }
            else { setData(null); setErr((r && r.message) || "读取失败") }
          })
        }, [kind])
        useEffect(() => { refresh() }, [refresh])

        useEffect(() => {
          if (!job || job.status !== "running") return
          const timer = setInterval(() => {
            call("GET", "/lifecycle/jobs/" + encodeURIComponent(job.id)).then((r) => {
              if (r && r.ok) setJob(r.job)
            })
          }, 1000)
          return () => clearInterval(timer)
        }, [job ? job.id : null, job ? job.status : null]) // eslint-disable-line react-hooks/exhaustive-deps

        useEffect(() => {
          if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight
        }, [job ? job.output.length : 0]) // eslint-disable-line react-hooks/exhaustive-deps

        function set(field, value) { setForm((f) => ({ ...f, [field]: value })) }
        function run(action) {
          const body = { action: action, ...form }
          if (action === "pack") body.with = form.with.split(",").map((s) => s.trim()).filter(Boolean)
          call("POST", "/lifecycle/run", body).then((r) => {
            if (r && r.ok) { setJob({ id: r.id, action: action, status: "running", output: [], exitCode: null }); setErr("") }
            else setErr((r && r.message) || "任务启动失败")
          })
        }

        const rows = Array.isArray(data) ? data : data && Array.isArray(data.worktrees) ? data.worktrees : []
        const columns = []
        for (const row of rows) {
          if (row && typeof row === "object") {
            for (const key of Object.keys(row)) if (!columns.includes(key)) columns.push(key)
          }
        }

        const fields = [
          { key: "with", label: "插件(逗号分隔)" },
          { key: "candidate", label: "候选 gen" },
          { key: "baseline", label: "基线 gen" },
          { key: "suite", label: "套件" },
          { key: "generation", label: "Generation" },
          { key: "eval", label: "EvalRun" },
          { key: "release", label: "Release" },
          { key: "target", label: "安装位(默认 VIVY_INSTALL_DIR)" },
        ]

        return h("div", { className: "vc-pane" }, [
          h("div", { className: "vc-card" }, [
            h("div", { className: "vc-card-title" }, "打包编译与版本管理（分发生命周期）"),
            h("div", { className: "vc-msg", style: { marginTop: 2 } }, "通过 vivy-studio.exe 对已钉选工作区执行 pack / eval / release / reject / install / rollback / inspect，并查看账本（generations / evals / releases / installs / events / worktrees）。本页与「总控台」的开发模式完全分离：不会启动或停止任何后端 / 前端开发进程。"),
          ]),
          h("div", { style: { display: "flex", gap: 6, alignItems: "center", flex: "none", flexWrap: "wrap" } }, [
            h("span", { className: "vc-key" }, "账本"),
            ...LEDGER_KINDS.map((k) =>
              h("button", { className: "vc-chip" + (k === kind ? " on" : ""), key: k, onClick: () => setKind(k) }, k),
            ),
            h("button", { className: "vc-btn", onClick: refresh }, "⟳ 刷新"),
          ]),
          err ? h("div", { className: "vc-msg" }, err) : null,
          rows.length === 0 && !err
            ? h("div", { className: "vc-empty" }, "（暂无记录）")
            : h("div", { style: { overflow: "auto", flex: "none", maxHeight: "30vh" } }, [
                h("table", { className: "vc-tbl" }, [
                  h("thead", null, [h("tr", null, columns.map((c) => h("th", { key: c }, c)))]),
                  h("tbody", null, rows.map((row, i) =>
                    h("tr", { key: i }, columns.map((c) => h("td", { key: c }, trunc(row && row[c], 80)))),
                  )),
                ]),
              ]),
          h("div", { className: "vc-card" }, [
            h("div", { className: "vc-grid" }, fields.map((f) =>
              h("label", { className: "vc-field", key: f.key }, [
                f.label,
                h("input", { className: "vc-input", value: form[f.key], onChange: (e) => set(f.key, e.target.value) }),
              ]),
            )),
            h("div", { style: { display: "flex", gap: 8, alignItems: "center", marginTop: 8, flexWrap: "wrap" } }, [
              ...Object.keys(ACTION_LABELS).map((a) =>
                h("button", {
                  className: "vc-btn" + (a === "release" ? " danger" : ""),
                  key: a,
                  disabled: !!job && job.status === "running",
                  onClick: () => run(a),
                }, ACTION_LABELS[a]),
              ),
              h("label", { style: { display: "flex", gap: 6, alignItems: "center", fontSize: 12, color: "var(--dsw-alias-state-warn-primary)" } }, [
                h("input", { type: "checkbox", checked: form.confirm, onChange: (e) => set("confirm", e.target.checked) }),
                "我确认发布：人工操作（--actor human --yes）",
              ]),
            ]),
            h("div", { className: "vc-msg", style: { marginTop: 6 } }, "发布必须勾选人工确认；install/rollback 目标由 vivy-studio.exe 拒绝源码树与 data/ 目录。"),
          ]),
          job
            ? h("div", { className: "vc-card" }, [
                h("div", { className: "vc-row" }, [
                  h("span", { className: "vc-key" }, "任务"),
                  h("span", { className: "vc-val" }, job.id + " · " + job.action + " · " + (job.status === "running" ? "运行中" : job.status === "ok" ? "成功" : "失败" + (job.exitCode != null ? " (exit " + job.exitCode + ")" : ""))),
                ]),
                h("pre", { ref: preRef, className: "vc-pre" }, job.output.length ? job.output.join("\n") : "（等待输出…）"),
                job.status !== "running" ? h("button", { className: "vc-btn", onClick: () => { setJob(null); refresh() } }, "关闭") : null,
              ])
            : null,
        ])
      }

      // ---- Root view ----
      const SECTIONS = [
        ["overview", "总控台"],
        ["logs", "日志"],
        ["package", "打包与版本"],
      ]

      function VivyConsoleView() {
        const [section, setSection] = useState("overview")
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
          section === "overview" ? h(OverviewPane, { status }) : null,
          section === "package" ? h(PackagePane, null) : null,
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