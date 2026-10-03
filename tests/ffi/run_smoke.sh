#!/usr/bin/env bash
# Build the shared library, launch the scripted provider, run the C smoke.
# Usage: tests/ffi/run_smoke.sh [go-binary-path]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GO="${1:-$(command -v go || echo "$HOME/toolchains/go/bin/go")}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"; kill ${MOCK_PID:-0} 2>/dev/null || true' EXIT

cd "$ROOT"
"$GO" build -tags vivy_headless -buildmode=c-shared -o "$WORK/vivy-shared.so" ./cmd/vivy-shared
cp cmd/vivy-shared/vivy_abi.h "$WORK/"

cat > "$WORK/smoke.yaml" <<EOF
server: {addr: "127.0.0.1:0"}
storage: {backend: sqlite, sqlite: {path: "$WORK/smoke.db"}}
providers: {active: deepseek}
runtime: {stream_buffer: 8, max_event_payload_bytes: 65536, workspace_root: "$WORK/ws"}
tools: {enabled: [write_note], approval: {expiration: "5m"}}
governance:
  profile: default
  profiles:
    default:
      default: allow
      rules: [{tool: write_note, decision: prompt}]
EOF

python3 tests/ffi/mock_provider.py 18321 &
MOCK_PID=$!
sleep 0.5

gcc -O0 -g -I"$WORK" -o "$WORK/smoke" tests/ffi/smoke.c "$WORK/vivy-shared.so" -lpthread
export DEEPSEEK_API_KEY=ffi-smoke-key VIVY_PROVIDER=deepseek VIVY_API_BASE="http://127.0.0.1:18321"
LD_LIBRARY_PATH="$WORK" "$WORK/smoke" "$WORK/smoke.yaml"
