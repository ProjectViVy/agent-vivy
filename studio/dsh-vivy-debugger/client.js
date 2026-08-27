// Vivy Studio gateway debugger — browser half.
// Injected inline into index.html by the Host plugin (webServer.tapIndex).
// Plain ES5-ish browser JS: creates a floating bottom-right button and a
// status/log panel, talking to /vivy-debugger/api/* over fetch. Uses the
// DSH theme CSS variables so it matches the shell skin.
(() => {
  if (window.__vivyDebugger) return
  window.__vivyDebugger = true

  var API = "/vivy-debugger/api"
  var stylesId = "dsh-vivy-debugger-style"

  if (!document.getElementById(stylesId)) {
    var style = document.createElement("style")
    style.id = stylesId
    style.textContent = [
      ".vdbg-fab{position:fixed;right:22px;bottom:22px;width:52px;height:52px;border-radius:50%;",
      "border:1px solid var(--dsw-alias-border-l2);background:var(--dsw-alias-bg-overlay);",
      "color:var(--dsw-alias-label-primary);cursor:pointer;font-size:21px;",
      "display:flex;align-items:center;justify-content:center;",
      "box-shadow:0 6px 20px rgba(0,0,0,.35);z-index:9998;pointer-events:auto;",
      "transition:transform .12s ease,box-shadow .12s ease}",
      ".vdbg-fab:hover{transform:translateY(-2px);box-shadow:0 9px 26px rgba(0,0,0,.45)}",
      ".vdbg-fabdot{position:absolute;top:5px;right:5px;width:10px;height:10px;border-radius:50%;",
      "background:var(--dsw-alias-state-error-primary);border:2px solid var(--dsw-alias-bg-overlay)}",
      ".vdbg-fabdot.on{background:var(--dsw-alias-state-success-primary)}",
      ".vdbg-fabdot.warn{background:var(--dsw-alias-state-warn-primary)}",
      ".vdbg-panel{position:fixed;right:22px;bottom:86px;width:470px;max-width:calc(100vw - 32px);",
      "max-height:min(66vh,640px);display:flex;flex-direction:column;",
      "background:var(--dsw-alias-bg-overlay);border:1px solid var(--dsw-alias-border-l2);",
      "border-radius:12px;box-shadow:0 12px 40px rgba(0,0,0,.35);",
      "color:var(--dsw-alias-label-primary);font-size:12px;",
      "font-family:system-ui,-apple-system,sans-serif;z-index:9999;pointer-events:auto;overflow:hidden}",
      ".vdbg-head{display:flex;align-items:center;gap:8px;padding:10px 12px;",
      "border-bottom:1px solid var(--dsw-alias-border-l1);flex:none}",
      ".vdbg-title{font-weight:600;font-size:13px}",
      ".vdbg-dot{width:9px;height:9px;border-radius:50%;background:var(--dsw-alias-state-error-primary);flex:none}",
      ".vdbg-dot.on{background:var(--dsw-alias-state-success-primary)}",
      ".vdbg-dot.warn{background:var(--dsw-alias-state-warn-primary)}",
      ".vdbg-close{margin-left:auto;background:none;border:none;color:var(--dsw-alias-label-secondary);",
      "cursor:pointer;font-size:15px;line-height:1}",
      ".vdbg-body{padding:10px 12px;overflow-y:auto;flex:1}",
      ".vdbg-row{display:flex;gap:6px;padding:2px 0}",
      ".vdbg-key{color:var(--dsw-alias-label-secondary);min-width:66px;flex:none}",
      ".vdbg-val{word-break:break-all;color:var(--dsw-alias-label-primary)}",
      ".vdbg-ctrl{display:flex;gap:8px;margin:10px 0 2px}",
      ".vdbg-btn{flex:1;height:30px;border:1px solid var(--dsw-alias-border-l2);border-radius:8px;",
      "background:var(--dsw-alias-bg-layer-1);color:var(--dsw-alias-label-primary);cursor:pointer;font-size:12px}",
      ".vdbg-btn:disabled{opacity:.45;cursor:default}",
      ".vdbg-btn.primary{border-color:var(--dsw-alias-brand-primary);color:var(--dsw-alias-brand-primary)}",
      ".vdbg-msg{min-height:16px;color:var(--dsw-alias-label-secondary);margin:4px 0}",
      ".vdbg-log{flex:1;min-height:150px;max-height:36vh;overflow:auto;margin:6px 0 0;padding:8px 10px;",
      "background:var(--dsw-alias-bg-base);border:1px solid var(--dsw-alias-border-l1);border-radius:8px;",
      "font-family:ui-monospace,Consolas,monospace;font-size:11px;line-height:1.55;",
      "white-space:pre-wrap;word-break:break-all;color:var(--dsw-alias-label-primary)}",
      ".vdbg-foot{display:flex;gap:6px;align-items:center;padding:8px 12px;",
      "border-top:1px solid var(--dsw-alias-border-l1);flex:none}",
      ".vdbg-input{flex:1;height:28px;padding:0 8px;background:var(--dsw-alias-bg-base);",
      "border:1px solid var(--dsw-alias-border-l1);border-radius:6px;color:var(--dsw-alias-label-primary);font-size:11px}",
    ].join("")
    document.head.appendChild(style)
  }

  function call(method, path, payload) {
    return fetch(API + path, {
      method: method,
      headers: payload ? { "content-type": "application/json" } : undefined,
      body: payload ? JSON.stringify(payload) : undefined,
      cache: "no-store",
    })
      .then(function (r) {
        return r.json()
      })
      .catch(function (err) {
        return { ok: false, message: String((err && err.message) || err) }
      })
  }

  var open = false
  var status = null
  var logs = []
  var busy = ""
  var msg = ""
  var exeInput = ""

  function el(tag, attrs, children) {
    var node = document.createElement(tag)
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        if (k === "className") node.className = attrs[k]
        else if (k === "text") node.textContent = attrs[k]
        else if (k === "title") node.title = attrs[k]
        else if (k === "value") node.value = attrs[k]
        else if (k === "disabled") node.disabled = attrs[k]
        else if (k.startsWith("on")) {
          node.addEventListener(k.slice(2).toLowerCase(), attrs[k])
        } else node.setAttribute(k, attrs[k])
      })
    }
    ;(children || []).forEach(function (c) {
      if (c != null) node.appendChild(typeof c === "string" ? document.createTextNode(c) : c)
    })
    return node
  }

  // Panel skeleton built once; refresh() updates only the dynamic parts so
  // the exe-path input keeps focus while the user types.
  var fab = null
  var fabDot = null
  var panel = null
  var headDot = null
  var rowsBox = null
  var startBtn = null
  var stopBtn = null
  var restartBtn = null
  var msgBox = null
  var logPre = null
  var inputEl = null

  function ensureDom() {
    if (fab) return
    fab = el("button", {
      className: "vdbg-fab",
      title: "Vivy 网关调试器：查看状态/日志，启动/停止/重启网关",
      onClick: function () {
        open = !open
        if (open) {
          panel.style.display = "flex"
          refresh()
        } else {
          panel.style.display = "none"
        }
      },
    })
    fab.textContent = "◈"
    fabDot = el("span", { className: "vdbg-fabdot" })
    fab.appendChild(fabDot)
    document.body.appendChild(fab)

    panel = el("div", { className: "vdbg-panel" })
    panel.style.display = "none"
    panel.appendChild(
      el("div", { className: "vdbg-head" }, [
        (headDot = el("span", { className: "vdbg-dot" })),
        el("span", { className: "vdbg-title", text: "Vivy 网关调试器" }),
        el("button", {
          className: "vdbg-close",
          text: "✕",
          onClick: function () {
            open = false
            panel.style.display = "none"
          },
        }),
      ]),
    )
    var body = el("div", { className: "vdbg-body" })
    rowsBox = el("div", { className: "vdbg-rows" })
    body.appendChild(rowsBox)
    body.appendChild(
      el("div", { className: "vdbg-ctrl" }, [
        (startBtn = el("button", {
          className: "vdbg-btn primary",
          onClick: function () {
            act("start", "启动")
          },
        })),
        (stopBtn = el("button", {
          className: "vdbg-btn",
          onClick: function () {
            act("stop", "停止")
          },
        })),
        (restartBtn = el("button", {
          className: "vdbg-btn",
          onClick: function () {
            act("restart", "重启")
          },
        })),
      ]),
    )
    msgBox = el("div", { className: "vdbg-msg" })
    body.appendChild(msgBox)
    logPre = el("pre", { className: "vdbg-log" })
    body.appendChild(logPre)
    panel.appendChild(body)

    inputEl = el("input", { className: "vdbg-input", value: exeInput })
    inputEl.placeholder = "vivy.exe 路径（留空自动解析）"
    inputEl.addEventListener("input", function () {
      exeInput = inputEl.value
    })
    panel.appendChild(
      el("div", { className: "vdbg-foot" }, [
        inputEl,
        el("button", {
          className: "vdbg-btn",
          text: "应用",
          onClick: function () {
            call("POST", "/setExe", { path: exeInput }).then(function (r) {
              msg = (r && r.ok && "已应用 EXE 路径") || "设置失败"
              refresh()
            })
          },
        }),
      ]),
    )
    document.body.appendChild(panel)
  }

  function refresh() {
    ensureDom()
    call("GET", "/status")
      .then(function (s) {
        status = s
        if (!exeInput) {
          call("GET", "/resolve").then(function (r) {
            if (r && r.exe) {
              exeInput = r.exe
              inputEl.value = exeInput
            }
          })
        }
        if (open) {
          call("GET", "/logs").then(function (l) {
            logs = (l && l.lines) || []
            logPre.textContent = logs.length ? logs.join("\n") : "（暂无日志）"
            logPre.scrollTop = logPre.scrollHeight
          })
        }
        paint()
      })
      .catch(function () {})
  }

  function act(name, label) {
    busy = label
    msg = ""
    paint()
    call("POST", "/" + name)
      .then(function (r) {
        msg = (r && r.message) || label + " 完成"
        busy = ""
        refresh()
      })
      .catch(function (err) {
        msg = label + " 失败: " + String((err && err.message) || err)
        busy = ""
        paint()
      })
  }

  function row(label, value) {
    return el("div", { className: "vdbg-row" }, [
      el("span", { className: "vdbg-key", text: label }),
      el("span", { className: "vdbg-val", text: String(value) }),
    ])
  }

  function paint() {
    ensureDom()
    var running = !!(status && status.running)
    var listening = !!(status && status.listening)
    fabDot.className = running ? "vdbg-fabdot on" : listening ? "vdbg-fabdot warn" : "vdbg-fabdot"
    headDot.className = running ? "vdbg-dot on" : listening ? "vdbg-dot warn" : "vdbg-dot"

    var pid = status && status.pid ? status.pid : "—"
    var started =
      status && status.startedAtMs ? new Date(status.startedAtMs).toLocaleTimeString() : "—"
    var uptime =
      status && status.startedAtMs
        ? Math.max(0, Math.floor((Date.now() - status.startedAtMs) / 1000)) + "s"
        : "—"
    var stateText = running
      ? "运行中 " + (status.managed ? "(本调试器管理)" : "(外部进程)")
      : listening
        ? "已停止（端口仍被监听）"
        : "已停止"

    rowsBox.textContent = ""
    ;[
      ["状态", stateText],
      ["PID", String(pid)],
      ["监听", status ? status.addr : "—"],
      ["启动于", started],
      ["运行时长", uptime],
      ["EXE", status && status.exe ? status.exe : "…"],
      ["配置", status && status.configPath ? status.configPath : "…"],
      ["日志", status && status.logPath ? status.logPath : "…"],
    ].forEach(function (pair) {
      rowsBox.appendChild(row(pair[0], pair[1]))
    })

    startBtn.textContent = busy === "启动" ? "启动中…" : "▶ 启动"
    stopBtn.textContent = busy === "停止" ? "停止中…" : "■ 停止"
    restartBtn.textContent = busy === "重启" ? "重启中…" : "⟳ 重启"
    startBtn.disabled = !!busy || running
    stopBtn.disabled = !!busy || !running
    restartBtn.disabled = !!busy

    msgBox.textContent =
      msg ||
      (status && status.isolated
        ? "隔离模式：数据在 data/studio-home/vivy-debugger，不触碰生产 Journal"
        : "")
  }

  function boot() {
    if (!document.body) {
      setTimeout(boot, 200)
      return
    }
    ensureDom()
    refresh()
    setInterval(function () {
      if (open) refresh()
      else {
        // keep the status dot fresh even when the panel is closed
        call("GET", "/status")
          .then(function (s) {
            status = s
            paint()
          })
          .catch(function () {})
      }
    }, 2000)
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot)
  else boot()
})()
