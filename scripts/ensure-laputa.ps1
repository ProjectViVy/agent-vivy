# Prepare the Git source closure used by Go and sealed go-host packaging.
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
    if ($head -ne $lock.commit) {
        $changes = Invoke-Git @("-C", $checkout, "status", "--porcelain")
        if ($changes) {
            throw "Laputa has local changes at $head; required commit is $($lock.commit). Commit or stash local changes, then retry just setup."
        }
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
}
if (-not $Quiet) {
    Write-Host "Laputa source ready at $($lock.commit): $checkout"
}
