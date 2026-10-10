# Prepare the pinned Laputa source closure required by Go builds.
[CmdletBinding()]
param(
    [string]$RepoRoot,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    # Windows PowerShell initializes PSScriptRoot after parameter defaults.
    $RepoRoot = Split-Path -Parent $PSScriptRoot
}
$root = (Resolve-Path -LiteralPath $RepoRoot).Path
$lock = Get-Content -Raw -LiteralPath (Join-Path $root "laputa-source.lock.json") | ConvertFrom-Json
if (-not $lock.repository -or $lock.commit -notmatch '^[0-9a-f]{40}$') {
    throw "laputa-source.lock.json must declare a repository and a full commit SHA"
}
$hostMod = Get-Content -Raw -LiteralPath (Join-Path $root "go.mod")
$revision = ([string]$lock.commit).Substring(0, 12)
foreach ($name in @("garden", "mentle", "laputa")) {
    $expected = [regex]::Escape("github.com/ProjectViVy/laputa/$name")
    if ($hostMod -notmatch "(?m)^\s*$expected\s+v0\.0\.0-\d{14}-$revision(?:\s|$)") {
        throw "Laputa $name Go revision in go.mod must match laputa-source.lock.json at $revision"
    }
}
if ($hostMod -notmatch '(?m)^go\s+(\S+)\s*$') {
    throw "go.mod must declare its Go version"
}
$goVersion = $Matches[1]
$checkout = Join-Path (Split-Path -Parent $root) "laputa"

function Invoke-Git([string[]]$Arguments) {
    $output = & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Laputa git command failed (exit $LASTEXITCODE). Check access to $($lock.repository), then retry just setup."
    }
    return $output
}

if (-not (Test-Path -LiteralPath $checkout)) {
    Write-Host "installing Laputa from $($lock.repository) at $($lock.commit)"
    Invoke-Git @("clone", "--no-checkout", $lock.repository, $checkout) | Out-Null
    Invoke-Git @("-C", $checkout, "checkout", "--detach", $lock.commit) | Out-Null
} else {
    if (-not (Test-Path -LiteralPath (Join-Path $checkout ".git"))) {
        throw "Laputa path already exists without a Git checkout: $checkout. Move it aside, then retry just setup."
    }
    $origin = (Invoke-Git @("-C", $checkout, "remote", "get-url", "origin")).Trim().TrimEnd('/') -replace '^git@github.com:', 'https://github.com/' -replace '\.git$', ''
    $expectedOrigin = ([string]$lock.repository).TrimEnd('/') -replace '\.git$', ''
    if ($origin -ne $expectedOrigin) {
        throw "Laputa origin is $origin; expected $expectedOrigin. Move the unrelated checkout aside, then retry just setup."
    }
    $head = Invoke-Git @("-C", $checkout, "rev-parse", "HEAD")
    $changes = Invoke-Git @("-C", $checkout, "status", "--porcelain")
    if ($changes) {
        throw "Laputa module source has local changes at $head; required commit is $($lock.commit). Commit or stash local changes, then retry just setup."
    }
    if ($head -ne $lock.commit) {
        Invoke-Git @("-C", $checkout, "fetch", "origin", $lock.commit) | Out-Null
        Invoke-Git @("-C", $checkout, "checkout", "--detach", $lock.commit) | Out-Null
    }
}

foreach ($name in @("garden", "mentle", "laputa")) {
    $modfile = Join-Path (Join-Path $checkout $name) "go.mod"
    if (-not (Test-Path -LiteralPath $modfile)) {
        throw "Laputa module is missing: $modfile. Check the pinned checkout before retrying."
    }
    $module = Get-Content -Raw -LiteralPath $modfile
    $expected = [regex]::Escape("github.com/ProjectViVy/laputa/$name")
    if ($module -notmatch "(?m)^module\s+$expected\s*$") {
        throw "Laputa module identity in $modfile must be github.com/ProjectViVy/laputa/$name. Preserve local changes and restore the pinned module declaration."
    }
    if ($module -notmatch '(?m)^go\s+(\S+)\s*$' -or $Matches[1] -ne $goVersion) {
        throw "Laputa Go version in $modfile must match host Go version $goVersion"
    }
}
if (-not $Quiet) {
    Write-Host "Laputa source ready at $($lock.commit): $checkout"
}
