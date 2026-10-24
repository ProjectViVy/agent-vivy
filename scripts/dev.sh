#!/usr/bin/env bash
# macOS/Linux split development loop: backend :8787 + Vite :3015.
# Windows uses dev.ps1 -Split and opens two windows; there is no portable
# "new window" here, so both processes run supervised in this terminal.
# Ctrl+C stops both. Usage: bash scripts/dev.sh [--no-browser]
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

backend_port=8787
ui_port=3015
open_browser=1
for arg in "$@"; do
  case "$arg" in
    --no-browser|-NoBrowser) open_browser=0 ;;
    --help|-h)
      echo "usage: scripts/dev.sh [--no-browser]"
      exit 0
      ;;
    *)
      echo "dev.sh: unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

for tool in go node pnpm; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "dev.sh: $tool is not on PATH" >&2
    exit 1
  fi
done

node scripts/ensure-laputa.mjs --quiet

# Go ignores the macOS system proxy, so pick up the caller's forward proxy or
# the local Clash/mihomo mixed port when it is listening.
export GOPROXY="${VIVY_GOPROXY:-${GOPROXY:-https://proxy.golang.org,direct}}"
go_forward_proxy="${VIVY_HTTPS_PROXY:-${HTTPS_PROXY:-}}"
if [[ -z "$go_forward_proxy" ]] && (exec 3<>"/dev/tcp/127.0.0.1/7897") 2>/dev/null; then
  go_forward_proxy="http://127.0.0.1:7897"
fi

# Use the Go version pinned by go.mod. A newer local Go can break pinned
# dependencies (for example x/net's Go 1.27 HTTP/2 switch vs grpc v1.64).
go_pin="$(sed -n 's/^go[[:space:]]\{1,\}\([0-9][^[:space:]]*\).*/\1/p' go.mod | head -n 1)"
if [[ -n "$go_pin" ]]; then
  export GOTOOLCHAIN="${GOTOOLCHAIN:-go$go_pin}"
fi

port_open() {
  (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

listen_pids() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null || true
}

# Match owners of a previous dev loop: the split backend binary (or a
# `go run ./cmd/vivy` artifact), and the Vite CLI behind a generic node process.
matches_dev_owner() {
  local label="$1" cmd="$2"
  local exe="${cmd%% *}"
  local base="${exe##*/}"
  case "$label" in
    "vivy backend")
      case "$base" in vivy|vivy.exe|vivy-backend|vivy-backend.exe) return 0 ;;
      esac
      ;;
    vite)
      case "$cmd" in *vite*) return 0 ;;
      esac
      ;;
  esac
  return 1
}

# Stop a previous loop started from this checkout, but never kill a process
# that merely happens to occupy the port.
stop_stale() {
  local port="$1" label="$2"
  port_open "$port" || return 0
  if ! command -v lsof >/dev/null 2>&1; then
    echo "dev.sh: 127.0.0.1:$port is already in use and lsof is unavailable; stop that process first" >&2
    exit 1
  fi
  local pids
  pids="$(listen_pids "$port")"
  for pid in $pids; do
    local cmd
    cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
    if ! matches_dev_owner "$label" "$cmd"; then
      echo "dev.sh: 127.0.0.1:$port is held by PID $pid ($cmd), which is not a $label process; stop it manually first" >&2
      exit 1
    fi
    echo "dev.sh: stopping previous $label (PID $pid)"
    kill -TERM "$pid" 2>/dev/null || true
  done
  local deadline=$((SECONDS + 10))
  while port_open "$port" && (( SECONDS < deadline )); do
    sleep 0.25
  done
  if port_open "$port"; then
    for pid in $pids; do kill -KILL "$pid" 2>/dev/null || true; done
    sleep 0.5
  fi
  if port_open "$port"; then
    echo "dev.sh: 127.0.0.1:$port is still in use after stopping the previous process; stop it manually and retry" >&2
    exit 1
  fi
}

stop_stale "$backend_port" "vivy backend"
stop_stale "$ui_port" "vite"

if [[ ! -d ui/node_modules ]]; then
  echo "dev.sh: installing ui dependencies"
  (cd ui && pnpm install --frozen-lockfile)
fi

if [[ -z "${VIVY_CONFIG:-}" && -z "${DEEPSEEK_API_KEY:-}" && -z "${OPENAI_API_KEY:-}" && -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "dev.sh: no provider API key; Vivy will start without a model; configure Settings -> Model"
fi

mkdir -p .workspace/dev
# Keep the dev loop off the daily ~/.vivy journal. A checkout at a different
# schema revision would otherwise fail closed on migration name drift.
export VIVY_USER_HOME="${VIVY_USER_HOME:-$root/.workspace/dev/home}"
mkdir -p "$VIVY_USER_HOME"
backend_exe=".workspace/dev/vivy-backend"
backend_log=".workspace/dev/backend.log"
vite_log=".workspace/dev/vite.log"
echo "dev.sh: building split-loop backend"
HTTP_PROXY="$go_forward_proxy" HTTPS_PROXY="$go_forward_proxy" \
  go build -tags vivy_headless -o "$backend_exe" ./cmd/vivy

: > "$backend_log"
: > "$vite_log"

# Job control puts each service in its own process group so cleanup can stop
# pnpm and the vite child it spawns together.
set -m
"$root/$backend_exe" >"$backend_log" 2>&1 &
backend_pid=$!
(cd ui && exec pnpm dev) >"$vite_log" 2>&1 &
ui_pid=$!
( tail -n +1 -f "$backend_log" 2>/dev/null | awk '{ print "[vivy] " $0; fflush() }' ) &
backend_tail=$!
( tail -n +1 -f "$vite_log" 2>/dev/null | awk '{ print "[vite] " $0; fflush() }' ) &
ui_tail=$!
set +m

stopping=0
cleanup() {
  trap - EXIT INT TERM
  stopping=1
  echo
  echo "dev.sh: stopping split loop"
  kill -TERM -- "-$ui_pid" "-$backend_pid" 2>/dev/null || true
  wait "$ui_pid" "$backend_pid" 2>/dev/null || true
  kill -TERM -- "-$ui_tail" "-$backend_tail" 2>/dev/null || true
  wait "$ui_tail" "$backend_tail" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

wait_port() {
  local port="$1" seconds="$2" label="$3" pid="$4" log="$5"
  local deadline=$((SECONDS + seconds))
  while (( SECONDS < deadline )); do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "dev.sh: $label exited before 127.0.0.1:$port was ready; see $log" >&2
      return 1
    fi
    if port_open "$port"; then
      return 0
    fi
    sleep 0.25
  done
  echo "dev.sh: $label did not listen on 127.0.0.1:$port within ${seconds}s; see $log" >&2
  return 1
}

wait_port "$backend_port" 90 "backend" "$backend_pid" "$backend_log"
wait_port "$ui_port" 45 "vite" "$ui_pid" "$vite_log"

url="http://127.0.0.1:$ui_port"
echo
echo "split loop ready: $url  (control plane http://127.0.0.1:$backend_port)"
echo "Ctrl+C stops both."
if (( open_browser )); then
  if [[ "$(uname -s)" == "Darwin" ]] && command -v open >/dev/null 2>&1; then
    open "$url" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$url" >/dev/null 2>&1 || true
  elif command -v wslview >/dev/null 2>&1; then
    wslview "$url" >/dev/null 2>&1 || true
  fi
fi

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$ui_pid" 2>/dev/null; do
  sleep 1
done
if (( stopping )); then
  exit 0
fi
if ! kill -0 "$backend_pid" 2>/dev/null; then
  echo "dev.sh: backend exited; see $backend_log" >&2
else
  echo "dev.sh: vite exited; see $vite_log" >&2
fi
exit 1
