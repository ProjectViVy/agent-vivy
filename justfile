# agent-vivy task recipes (requires `just`; otherwise run the go commands directly)

set shell := ["powershell.exe", "-NoProfile", "-Command"]
set windows-shell := ["powershell.exe", "-NoProfile", "-Command"]

go := "go"

# Persist GOPROXY mirror (proxy.golang.org is unreachable) and download deps
setup:
    {{go}} env -w GOPROXY=https://goproxy.cn,direct
    {{go}} mod download

build:
    {{go}} build ./...

test:
    {{go}} test ./...

vet:
    {{go}} vet ./...

fmt-check:
    powershell -NoProfile -Command '$files = gofmt -l .; if ($files) { Write-Output $files; exit 1 }'

ci: fmt-check vet test

run:
    {{go}} run ./cmd/vivy

# Packer only. Not the daily gateway.
sdk:
    {{go}} build -o vivy-sdk.exe ./sdk

# Studio lifecycle tool. Owns the Studio ledger and lifecycle ops. Not the daily gateway.
studio:
    {{go}} build -o vivy-studio.exe ./cmd/vivy-studio
