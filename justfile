# agent-vivy task recipes (requires `just`; otherwise run the go commands directly)

set shell := ["powershell.exe", "-NoProfile", "-Command"]
set windows-shell := ["powershell.exe", "-NoProfile", "-Command"]

# Use the toolchain selected on PATH. This lets actions/setup-go enforce the
# go.mod version instead of silently using a preinstalled Windows copy.
go := "go"
gofmt := "gofmt"
vivy_code := if os() == "windows" { "vivy-code.exe" } else { "vivy-code" }

# Persist GOPROXY mirror (proxy.golang.org is unreachable) and download deps
setup:
    & "{{go}}" env -w GOPROXY=https://goproxy.cn,direct
    & "{{go}}" mod download

build:
    & "{{go}}" build ./...

test:
    & "{{go}}" test ./...

vet:
    & "{{go}}" vet ./...

fmt-check:
    powershell -NoProfile -Command '$files = & git ls-files -- ''*.go''; if ($LASTEXITCODE) { exit $LASTEXITCODE }; $unformatted = $files | ForEach-Object { & ''{{gofmt}}'' -l $_ }; if ($unformatted) { Write-Output $unformatted; exit 1 }'

# Per-module vet+test for plugins/* and faces/* independent modules (each
# with its own go.mod; hello-fs belongs to the main module and is covered
# by vet/test). No artifact builds — packing stays the vivy-sdk five-step
# path.
plugin-ci:
    powershell -NoProfile -Command '$fail = 0; foreach ($root in @(''plugins'', ''faces'')) { if (-not (Test-Path $root)) { continue }; $mods = Get-ChildItem $root -Directory | Where-Object { Test-Path (Join-Path $_.FullName ''go.mod'') }; foreach ($m in $mods) { Write-Output (''== plugin-ci: '' + $root + ''/'' + $m.Name); Push-Location $m.FullName; & ''{{go}}'' vet ./...; if ($LASTEXITCODE) { $fail = 1 }; & ''{{go}}'' test ./...; if ($LASTEXITCODE) { $fail = 1 }; Pop-Location } }; exit $fail'

ui-ci:
    Set-Location ui; pnpm install --frozen-lockfile; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm typecheck; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm test; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm build

headless-compile:
    & "{{go}}" test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui

build-split:
    New-Item -ItemType Directory -Force -Path dist | Out-Null
    & "{{go}}" build -tags vivy_headless -o dist/vivy-backend.exe ./cmd/vivy; if ($LASTEXITCODE) { exit $LASTEXITCODE }
    Set-Location ui; pnpm build -- --outDir ../dist/vivy-ui --emptyOutDir

# ui-ci first: the embedded-UI package (ui/embed.go, go:embed all:dist)
# cannot compile on a fresh checkout until the Vite build creates ui/dist,
# and a committed ui/dist/.keep is not an option because pnpm's
# emptyOutDir wipes it on every build.
ci: fmt-check ui-ci vet test headless-compile plugin-ci

ui-e2e:
    Set-Location ui; pnpm build; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm e2e

run:
    & "{{go}}" run ./cmd/vivy

# Build the independent VIVY CODE terminal product.
vivy-code:
    & "{{go}}" build -tags vivy_headless -o "{{vivy_code}}" ./cmd/vivy-code

# Start an independent VIVY CODE instance in the current project.
tui: vivy-code
    & "./{{vivy_code}}"

# One-click split loop: backend :8787 + Vite :3015. Ctrl+C stops both.
dev:
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./dev.ps1

# Container packaging of the default embedded-UI binary. Not part of just ci.
docker-build:
    docker build -t vivy:local --build-arg GOPROXY=https://goproxy.cn,direct --build-arg NPM_REGISTRY=https://registry.npmmirror.com .

docker-up:
    docker compose up --build -d

docker-up-postgres:
    docker compose -f docker-compose.yml -f docker-compose.postgres.yml up --build -d

# Optional. Skips when VIVY_POSTGRES_TEST_DSN is unset. Not part of just ci.
test-postgres:
    if (-not $env:VIVY_POSTGRES_TEST_DSN) { Write-Output 'VIVY_POSTGRES_TEST_DSN unset; skipping'; exit 0 }; & "{{go}}" test ./internal/storage/postgres

# Packer only. Not the daily gateway.
sdk:
    & "{{go}}" build -o vivy-sdk.exe ./sdk

# Studio lifecycle tool. Owns the Studio ledger and lifecycle ops. Not the daily gateway.
studio:
    & "{{go}}" build -o vivy-studio.exe ./cmd/vivy-studio

# Ensure git submodule studio/ (ProjectViVy/vivy-studio) is checked out.
# Safe no-op when already present. Used by launch-vivy-studio.ps1 and agents.
ensure-studio:
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./scripts/ensure-studio.ps1

# Optional bootstrap: Go deps + studio shell submodule.
setup-all: setup ensure-studio
