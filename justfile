# agent-vivy task recipes (requires `just`; otherwise run the go commands directly)

set shell := ["powershell.exe", "-NoProfile", "-Command"]
set windows-shell := ["powershell.exe", "-NoProfile", "-Command"]

go := if os() == "windows" { "C:/Program Files/Go/bin/go.exe" } else { "go" }
gofmt := if os() == "windows" { "C:/Program Files/Go/bin/gofmt.exe" } else { "gofmt" }

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
    powershell -NoProfile -Command '$files = rg --files cmd internal sdk ui plugins -g ''*.go''; $unformatted = $files | ForEach-Object { & ''{{gofmt}}'' -l $_ }; if ($unformatted) { Write-Output $unformatted; exit 1 }'

# Per-module vet+test for plugins/* independent modules (each with its own
# go.mod; hello-fs belongs to the main module and is covered by vet/test).
# No artifact builds — packing stays the vivy-sdk five-step path.
plugin-ci:
    powershell -NoProfile -Command '$mods = Get-ChildItem plugins -Directory | Where-Object { Test-Path (Join-Path $_.FullName ''go.mod'') }; $fail = 0; foreach ($m in $mods) { Write-Output (''== plugin-ci: '' + $m.Name); Push-Location $m.FullName; & ''{{go}}'' vet ./...; if ($LASTEXITCODE) { $fail = 1 }; & ''{{go}}'' test ./...; if ($LASTEXITCODE) { $fail = 1 }; Pop-Location }; exit $fail'

ui-ci:
    Set-Location ui; pnpm install --frozen-lockfile; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm typecheck; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm test; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm build

headless-compile:
    & "{{go}}" test -run '^$' -tags vivy_headless ./cmd/vivy ./ui

build-split:
    New-Item -ItemType Directory -Force -Path dist | Out-Null
    & "{{go}}" build -tags vivy_headless -o dist/vivy-backend.exe ./cmd/vivy; if ($LASTEXITCODE) { exit $LASTEXITCODE }
    Set-Location ui; pnpm build -- --outDir ../dist/vivy-ui --emptyOutDir

ci: fmt-check vet test headless-compile plugin-ci ui-ci

ui-e2e:
    Set-Location ui; pnpm build; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm e2e

run:
    & "{{go}}" run ./cmd/vivy

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
