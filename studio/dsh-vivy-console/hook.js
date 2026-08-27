// Vivy Studio console — frontend-debug bridge (hook.js).
//
// Injected by the /vivy-web proxy into the VIVY WEB index.html before the
// app bundle loads (or loaded directly when the VIVY WEB is opened as a
// standalone tab), so it captures console output, page errors, and
// WebSocket (JSON-RPC) traffic from the very first moment, and relays it to
// the Studio console tab via postMessage.
//
// Relay targets: when the VIVY WEB is embedded as an iframe the receiver is
// `window.parent`; when it is opened as a standalone tab via window.open the
// receiver is `window.opener` (the console tab that opened it). Both are
// posted to so either embedding works.
//
// Also answers console-tab commands: "ping" and "evaluate" (run a JavaScript
// expression in the VIVY WEB page and return a JSON-safe result).
//
// Safety: every URL that leaves this page has its RPC token redacted, and
// WebSocket frame bodies are capped and secret-looking keys masked — the
// Studio UI never sees raw tokens or unbounded payloads (D-010).
(() => {
  if (window.__vivyConsoleHook) return
  window.__vivyConsoleHook = true

  var HOOK_TAG = true
  var CAP = 220
  var SECRET_KEYS = /(token|authorization|api[_-]?key|secret|password|credential)/i

  function relayTargets() {
    var targets = []
    try {
      if (window.parent && window.parent !== window) targets.push(window.parent)
    } catch (e) {
      /* cross-origin parent — cannot post */
    }
    try {
      if (window.opener && window.opener !== window && window.opener !== window.parent) targets.push(window.opener)
    } catch (e) {
      /* cross-origin opener — cannot post */
    }
    return targets
  }

  function post(type, payload) {
    var targets = relayTargets()
    if (targets.length === 0) return
    var message = { __vivyConsole: HOOK_TAG, type: type, payload: payload, url: location.href, t: Date.now() }
    for (var i = 0; i < targets.length; i++) {
      try {
        targets[i].postMessage(message, "*")
      } catch (e) {
        /* receiver gone — ignore */
      }
    }
  }

  function clip(value) {
    var text = typeof value === "string" ? value : String(value)
    if (text.length > CAP) return text.slice(0, CAP) + "…"
    return text
  }

  function redact(text) {
    return String(text).replace(/([?&](?:token|key|sig)=)[^&#]+/gi, "$1***")
  }

  function maskSecrets(text) {
    var value = String(text)
    try {
      var parsed = JSON.parse(value)
      return JSON.stringify(maskObject(parsed))
    } catch (e) {
      return value.replace(/(["']?(?:token|authorization|api[_-]?key|secret|password)["']?\s*[:=]\s*["']?)[^"',;\s}]+/gi, "$1***")
    }
  }

  function maskObject(node) {
    if (Array.isArray(node)) return node.map(maskObject)
    if (node && typeof node === "object") {
      var out = {}
      for (var key in node) {
        if (Object.prototype.hasOwnProperty.call(node, key)) {
          out[key] = SECRET_KEYS.test(key) ? "***" : maskObject(node[key])
        }
      }
      return out
    }
    return node
  }

  function stringifyArg(value) {
    if (typeof value === "string") return clip(value)
    if (value instanceof Error) return clip(value.message || String(value))
    try {
      return clip(JSON.stringify(value))
    } catch (e) {
      return clip(String(value))
    }
  }

  // ---- console capture ----
  var LEVELS = ["log", "info", "warn", "error", "debug"]
  for (var i = 0; i < LEVELS.length; i++) {
    ;(function (level) {
      var original = console[level]
      console[level] = function () {
        try {
          var args = []
          for (var a = 0; a < arguments.length && a < 6; a++) args.push(stringifyArg(arguments[a]))
          post("console", { level: level, args: args })
        } catch (e) {
          /* never break the app over capture */
        }
        return original.apply(console, arguments)
      }
    })(LEVELS[i])
  }

  window.addEventListener("error", function (event) {
    post("pageerror", {
      message: clip(event.message || String(event.error || "")),
      file: event.filename,
      line: event.lineno,
      col: event.colno,
    })
  })
  window.addEventListener("unhandledrejection", function (event) {
    post("unhandledrejection", { message: clip(String(event.reason && event.reason.message ? event.reason.message : event.reason)) })
  })

  // ---- WebSocket (JSON-RPC) capture ----
  var NativeWebSocket = window.WebSocket
  function patchedWebSocket(url, protocols) {
    var instance
    try {
      instance = protocols === undefined ? new NativeWebSocket(url) : new NativeWebSocket(url, protocols)
    } catch (error) {
      throw error
    }
    var safeUrl = redact(String(url))
    post("ws", { dir: "open", url: safeUrl })
    instance.addEventListener("open", function () {
      post("ws", { dir: "opened", url: safeUrl })
    })
    instance.addEventListener("message", function (event) {
      var text = ""
      try {
        text = typeof event.data === "string" ? event.data : String(event.data)
      } catch (e) {
        text = ""
      }
      post("wsmsg", { dir: "recv", url: safeUrl, body: maskSecrets(clip(text)) })
    })
    var originalSend = instance.send.bind(instance)
    instance.send = function (data) {
      try {
        var text = typeof data === "string" ? data : String(data)
        post("wsmsg", { dir: "send", url: safeUrl, body: maskSecrets(clip(text)) })
      } catch (e) {
        /* keep sending */
      }
      return originalSend(data)
    }
    return instance
  }
  if (typeof NativeWebSocket === "function") {
    window.WebSocket = patchedWebSocket
    window.WebSocket.prototype = NativeWebSocket.prototype
    window.WebSocket.CONNECTING = NativeWebSocket.CONNECTING
    window.WebSocket.OPEN = NativeWebSocket.OPEN
    window.WebSocket.CLOSING = NativeWebSocket.CLOSING
    window.WebSocket.CLOSED = NativeWebSocket.CLOSED
  }

  // ---- parent commands ----
  window.addEventListener("message", function (event) {
    var data = event.data
    if (!data || data.__vivyConsole !== "cmd") return
    if (data.cmd === "ping") {
      post("pong", {})
      return
    }
    if (data.cmd === "evaluate") {
      var ok = false
      var value
      try {
        value = new Function("return (" + String(data.expr || "") + ")")()
        ok = true
      } catch (error) {
        value = String((error && error.message) || error)
      }
      var safe
      try {
        safe = JSON.stringify(value)
        if (safe === undefined) safe = String(value)
      } catch (error) {
        safe = String(value)
      }
      post("eval-result", { id: data.id, ok: ok, value: safe === undefined ? "undefined" : safe })
    }
  })

  post("ready", {})
})()
