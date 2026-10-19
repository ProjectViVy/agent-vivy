# agent-vivy task recipes. One entry point for Windows, WSL, and Linux.
# Windows-only process management stays in dev.ps1; build logic that needs
# branching lives in cross-platform Node helpers under scripts/.

set windows-shell := ["powershell.exe", "-NoProfile", "-Command"]

go := "go"
gofmt := "gofmt"
vivy_code := if os() == "windows" { "vivy-code.exe" } else { "vivy-code" }
vivy_sdk := if os() == "windows" { "vivy-sdk.exe" } else { "vivy-sdk" }
vivy_studio := if os() == "windows" { "vivy-studio.exe" } else { "vivy-studio" }
dev_cmd := if os() == "windows" {
    "powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./dev.ps1"
} else {
    "bash ./scripts/dev.sh"
}
ensure_studio_cmd := if os() == "windows" {
    "powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./scripts/ensure-studio.ps1"
} else {
    "git submodule update --init --recursive -- studio"
}

# Project-level default. An existing GOPROXY wins; VIVY_GOPROXY overrides both.
# Recipes never write `go env -w`, so a mirror cannot leak into other projects.
export GOPROXY := env_var_or_default("VIVY_GOPROXY", env_var_or_default("GOPROXY", "https://goproxy.cn,direct"))

# Diagnose tools, pins, proxy, ports, write access, and UI artifacts.
# Read-only: it never changes global settings.
doctor:
    node scripts/doctor.mjs

# Prepare the pinned Laputa source closure shared by local development and CI.
ensure-laputa:
    node scripts/ensure-laputa.mjs --quiet

diva-recall-test: ensure-laputa
    node scripts/diva-recall.mjs

bootstrap-test:
    node --test scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs scripts/bml-ci.test.mjs

# Prepare the sibling Laputa source, then download modules with a
# process-local proxy. VIVY_GOPROXY overrides the project default.
setup:
    node scripts/setup.mjs

build: ensure-laputa
    go build ./...

test: ensure-laputa
    # The Windows runtime package exercises hundreds of SQLite-backed real paths
    # and now legitimately exceeds Go's default 10-minute per-package timeout;
    # the SDK's pack/eval suite (sdk/internal) builds ~20 temporary modules and
    # exceeds it too. internal/runtime crossed the 20-minute bound on the
    # Windows runner, so the shared bound is 35m.
    # Memory-loop integration requires the DIVA Recipe, not the default body.
    # The second pass runs every excluded test against its sealed Assembly.
    # Serialize real SQLite fixture packages to avoid Windows flush contention.
    go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...
    node scripts/memory-loop.mjs

# Explicit SDK publication-evidence check, separate from ordinary development
# and CI behavior tests. A changed source tree is not a verified release.
release-conformance: ensure-laputa
    node scripts/release-conformance.mjs

vet: ensure-laputa
    go vet ./...

fmt-check:
    node scripts/fmt-check.mjs

# Per-module vet+test for plugins/* and faces/* independent modules (each
# with its own go.mod; hello-fs belongs to the main module and is covered
# by vet/test). No artifact builds — packing stays the vivy-sdk five-step
# path.
plugin-ci:
    node scripts/plugin-ci.mjs

# Root ./... excludes this independently declared nested module.
bml-ci:
    node scripts/bml-ci.mjs "{{go}}"

# Build the assets required by go:embed without coupling backend checks to the
# UI typecheck/test gate. CI jobs intentionally use separate installations.
ui-build: ensure-laputa
    pnpm --dir ui install --frozen-lockfile
    pnpm --dir ui build

ui-core: ensure-laputa
    pnpm --dir ui install --frozen-lockfile
    pnpm --dir ui typecheck
    pnpm --dir ui test
    pnpm --dir ui build

# Uses the locked TypeScript parser installed by ui-core.
i18n-check: ui-core
    node scripts/check-i18n-completeness.js
    node --test scripts/check-i18n-cross-face.test.js
    node scripts/check-i18n-cross-face.js

# Complete standalone UI gate, including cross-face I18N conformance.
ui-ci: i18n-check

headless-compile: ensure-laputa
    go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui

build-split: ensure-laputa
    node scripts/build-split.mjs

# ui-ci first: the embedded-UI package (ui/embed.go, go:embed all:dist)
# cannot compile on a fresh checkout until the Vite build creates ui/dist,
# and a committed ui/dist/.keep is not an option because pnpm's
# emptyOutDir wipes it on every build.
ci: ensure-laputa bootstrap-test fmt-check ui-ci vet test diva-recall-test headless-compile plugin-ci bml-ci

# Independent backend gate for Actions. It builds ui/dist for go:embed but
# leaves UI typechecking and tests to ui-ci so both lanes always report.
backend-ci: ensure-laputa bootstrap-test fmt-check ui-build vet test diva-recall-test headless-compile plugin-ci bml-ci

ui-e2e:
    pnpm --dir ui build
    pnpm --dir ui e2e

# Reproducible native Toolkit gate: pack and inspect the default Recipe, then
# run both Toolkit Playwright suites on their own backend, Vite server and data
# root. Override with VIVY_TOOLKITS_E2E_BACKEND_PORT / VIVY_TOOLKITS_E2E_UI_PORT.
toolkits-e2e:
    node scripts/toolkits-e2e.mjs

# Fast local checkpoint before a full gate: formatting, vet, and compilation.
check-fast: fmt-check vet build

# Run the backend with the isolated development home and port (8788 by
# default). The daily Vivy home (~/.vivy) and the repo's data/ stay untouched.
run: ensure-laputa
    node scripts/run-dev.mjs

# Full race lane. Not part of the default Windows CI gate because the race
# detector needs a C toolchain.
test-race: ensure-laputa
    go test -race -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...

# PR race lane: the kernel packages, with the UI embed built first. Full-tree
# racing stays available as test-race.
race-ci: ensure-laputa ui-build
    go test -race -p 1 -timeout 45m -skip '^TestMemoryLoop' ./internal/...

# Build the independent VIVY CODE terminal product.
vivy-code: ensure-laputa
    go build -tags vivy_headless -o "{{vivy_code}}" ./cmd/vivy-code

# Start an independent VIVY CODE instance in the current project.
tui: vivy-code
    ./{{vivy_code}}

# One-click split loop. Defaults: backend 127.0.0.1:8788, Vite 3016.
dev *args:
    {{dev_cmd}} {{args}}

# Same prep as dev, but backend and Vite run in two separate windows.
[windows]
dev-vivy:
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./dev.ps1 -Split

[unix]
dev-vivy:
    @echo "dev-vivy is Windows-only; use 'just dev' or two terminals"

# Container packaging of the default embedded-UI binary. Not part of just ci.
# scripts/docker-build.mjs passes the pinned ../laputa checkout as a named
# build context so local replace directives resolve inside the image.
docker-build *args: ensure-laputa
    node scripts/docker-build.mjs {{args}}

docker-up: ensure-laputa
    docker compose up --build -d

docker-up-postgres: ensure-laputa
    docker compose -f docker-compose.yml -f docker-compose.postgres.yml up --build -d

# Fails closed when VIVY_POSTGRES_TEST_DSN is unset. A skipped PostgreSQL
# package test is not a passing gate.
test-postgres: ensure-laputa
    node scripts/test-postgres.mjs

# Packer only. Not the daily gateway.
sdk: ensure-laputa
    go build -o "{{vivy_sdk}}" ./sdk

# Studio lifecycle tool. Owns the Studio ledger and lifecycle ops. Not the daily gateway.
studio: ensure-laputa
    go build -o "{{vivy_studio}}" ./cmd/vivy-studio

# Ensure git submodule studio/ (ProjectViVy/vivy-studio) is checked out.
# Safe no-op when already present. Used by launch-vivy-studio.ps1 and agents.
ensure-studio:
    {{ensure_studio_cmd}}

# Optional bootstrap: Go deps + studio shell submodule.
setup-all: setup ensure-studio

# Create .worktrees/<slug>/agent-vivy plus its own pinned
# .worktrees/<slug>/laputa checkout, so parallel lanes cannot flip each
# other's Laputa revision.
lane-new slug:
    node scripts/lane-new.mjs {{slug}}
