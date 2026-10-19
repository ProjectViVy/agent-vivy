#!/usr/bin/env bash
# Split development loop for WSL/Linux: backend + Vite, isolated from the daily
# Vivy home. Ctrl+C stops both process groups.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

defaults="scripts/dev.defaults.json"
read_default() {
  node -e "const d=require('./$defaults');process.stdout.write(String(d['$1']))"
}

backend_port="${VIVY_DEV_BACKEND_PORT:-$(read_default backendPort)}"
ui_port="${VIVY_DEV_UI_PORT:-$(read_default uiPort)}"
dev_home="$root/$(read_default userHome)"
open_browser=0
for arg in "$@"; do
  case "$arg" in
    --open) open_browser=1 ;;
    --no-browser|-NoBrowser) open_browser=0 ;;
    --help|-h)
      echo "usage: scripts/dev.sh [--open]"
      exit 0
      ;;
    *)
      echo "dev.sh: unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

for tool in go node pnpm git; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "dev.sh: $tool is not on PATH" >&2
    exit 1
  fi
done

node scripts/ensure-laputa.mjs --quiet

export GOPROXY="${VIVY_GOPROXY:-${GOPROXY:-https://goproxy.cn,direct}}"
mkdir -p "$dev_home" .workspace/dev
export VIVY_USER_HOME="$dev_home"
export VIVY_ADDR="127.0.0.1:$backend_port"
export VIVY_CONFIG="${VIVY_DEV_CONFIG:-$root/scripts/dev.config.yaml}"
export VIVY_UI_PORT="$ui_port"
export VIVY_BACKEND_ADDR="http://127.0.0.1:$backend_port"

if [[ ! -d ui/node_modules ]]; then
  echo "dev.sh: installing ui dependencies"
  (cd ui && pnpm install --frozen-lockfile)
fi

echo "dev.sh: building split-loop backend"
go build -tags vivy_headless -o .workspace/dev/vivy-backend ./cmd/vivy

port_in_use() {
  (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

for port in "$backend_port" "$ui_port"; do
  if port_in_use "$port"; then
    echo "dev.sh: 127.0.0.1:$port is already in use; stop that process first" >&2
    if command -v ss >/dev/null 2>&1; then
      ss -ltnp 2>/dev/null | grep ":$port " || true
    fi
    exit 1
  fi
done

wait_port() {
  local port="$1" seconds="$2" label="$3" pid="$4"
  local deadline=$((SECONDS + seconds))
  while (( SECONDS < deadline )); do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "dev.sh: $label exited before 127.0.0.1:$port was ready" >&2
      return 1
    fi
    if port_in_use "$port"; then
      return 0
    fi
    sleep 0.25
  done
  echo "dev.sh: $label did not listen on 127.0.0.1:$port within ${seconds}s" >&2
  return 1
}

if command -v setsid >/dev/null 2>&1; then
  setsid "$root/.workspace/dev/vivy-backend" &
  backend_pid=$!
  (cd ui && exec setsid pnpm dev) &
  ui_pid=$!
else
  "$root/.workspace/dev/vivy-backend" &
  backend_pid=$!
  (cd ui && exec pnpm dev) &
  ui_pid=$!
fi

cleanup() {
  trap - EXIT INT TERM
  kill -TERM -- "-$ui_pid" "-$backend_pid" 2>/dev/null || true
  wait "$ui_pid" "$backend_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

if ! wait_port "$backend_port" 90 backend "$backend_pid"; then
  exit 1
fi
if ! wait_port "$ui_port" 45 vite "$ui_pid"; then
  exit 1
fi

echo
echo "split loop ready: http://127.0.0.1:$ui_port  (control plane http://127.0.0.1:$backend_port)"
echo "data root: $dev_home"
echo "Ctrl+C stops both."

if [[ "$open_browser" -eq 1 ]]; then
  if command -v wslview >/dev/null 2>&1; then
    wslview "http://127.0.0.1:$ui_port" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "http://127.0.0.1:$ui_port" >/dev/null 2>&1 || true
  fi
fi

wait "$backend_pid" "$ui_pid"
