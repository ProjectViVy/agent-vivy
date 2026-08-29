<#
.SYNOPSIS
  Ensure the vivy-studio git submodule at studio/ is checked out.

.DESCRIPTION
  The Studio shell and plugins live in ProjectViVy/vivy-studio and are mounted
  here as submodule path `studio/`. A plain `git clone` without
  `--recurse-submodules` leaves an empty directory. This script detects a
  missing checkout and runs `git submodule update --init --recursive -- studio`.

  Safe to call repeatedly (no-op when the sentinel package.json is present).
  Does NOT run `git submodule add` — the host repo must already list studio in
  .gitmodules. Does not touch tenant journals or data/vivy.db.

.PARAMETER RepoRoot
  agent-vivy repository root. Default: parent of this script's directory,
  or current directory when invoked as a module helper.

.PARAMETER Quiet
  Suppress the "already present" message.

.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/ensure-studio.ps1
#>
[CmdletBinding()]
param(
  [string]$RepoRoot = "",
  [switch]$Quiet
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot([string]$start) {
  if ($start -and (Test-Path (Join-Path $start ".git"))) { return (Resolve-Path $start).Path }
  if ($start -and (Test-Path (Join-Path $start "launch-vivy-studio.ps1"))) { return (Resolve-Path $start).Path }

  $dir = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
  for ($i = 0; $i -lt 8; $i++) {
    $hasLaunch = Test-Path (Join-Path $dir "launch-vivy-studio.ps1")
    $hasGit = (Test-Path (Join-Path $dir ".git")) -or (Test-Path (Join-Path $dir ".gitmodules"))
    if ($hasLaunch -and $hasGit) { return $dir }
    $parent = Split-Path -Parent $dir
    if (-not $parent -or $parent -eq $dir) { break }
    $dir = $parent
  }
  throw "cannot resolve agent-vivy repo root; pass -RepoRoot"
}

$root = Resolve-RepoRoot $RepoRoot
Set-Location $root

$studioDir = Join-Path $root "studio"
$sentinel = Join-Path $studioDir "dsh-vivy-studio\package.json"
$gitmodules = Join-Path $root ".gitmodules"

function Test-StudioReady {
  return (Test-Path -LiteralPath $sentinel)
}

if (Test-StudioReady) {
  if (-not $Quiet) {
    Write-Host "studio submodule OK ($studioDir)"
  }
  exit 0
}

if (-not (Test-Path -LiteralPath $gitmodules)) {
  throw "studio shell missing and .gitmodules not found under $root. Upgrade/clone agent-vivy first."
}

$gm = Get-Content -Raw -LiteralPath $gitmodules
if ($gm -notmatch '(?m)^\s*path\s*=\s*studio\s*$' -and $gm -notmatch 'submodule "studio"') {
  throw @"
studio shell is missing and .gitmodules has no [submodule `"studio`"] entry.
This checkout is too old or incomplete. Pull agent-vivy main (or the branch
that added the vivy-studio submodule), then re-run:

  just ensure-studio
  # or: git submodule update --init --recursive -- studio
"@
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  throw "git is required to install the studio submodule but was not found on PATH"
}

Write-Host "studio submodule not installed (missing $sentinel)"
Write-Host "running: git submodule update --init --recursive -- studio"

& git -C $root submodule update --init --recursive -- studio
if ($LASTEXITCODE -ne 0) {
  throw @"
git submodule update failed (exit $LASTEXITCODE).
Check network access to https://github.com/ProjectViVy/vivy-studio.git and retry:

  git submodule update --init --recursive -- studio
  # or: just ensure-studio
"@
}

if (-not (Test-StudioReady)) {
  throw @"
studio submodule update finished but sentinel still missing:
  $sentinel
Inspect `git submodule status` and the studio/ directory.
"@
}

Write-Host "studio submodule installed -> $studioDir"
exit 0
