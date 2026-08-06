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
    powershell -NoProfile -Command "$files = gofmt -l .; if ($files) { Write-Output $files; exit 1 }"

ci: fmt-check vet test

run:
    {{go}} run ./cmd/vivy
