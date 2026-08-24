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
    powershell -NoProfile -Command '$files = rg --files cmd internal sdk ui -g ''*.go''; $unformatted = $files | ForEach-Object { & ''{{gofmt}}'' -l $_ }; if ($unformatted) { Write-Output $unformatted; exit 1 }'

ui-ci:
    Set-Location ui; pnpm install --frozen-lockfile; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm typecheck; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm test; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm build

ci: fmt-check vet test ui-ci

ui-e2e:
    Set-Location ui; pnpm build; if ($LASTEXITCODE) { exit $LASTEXITCODE }; pnpm e2e

run:
    & "{{go}}" run ./cmd/vivy

# Packer only. Not the daily gateway.
sdk:
    & "{{go}}" build -o vivy-sdk.exe ./sdk

# Studio lifecycle tool. Owns the Studio ledger and lifecycle ops. Not the daily gateway.
studio:
    & "{{go}}" build -o vivy-studio.exe ./cmd/vivy-studio
