<#
.SYNOPSIS
  Restart the running Vivy Studio dsh server so skin/plugin file changes take effect.

.DESCRIPTION
  The Studio server caches skin files (theme.css, brand.js, index.js) in memory at
  boot — editing them does NOT hot-reload. This script kills the listener on the
  Studio port and relaunches the same profile with an explicit environment
  (DSH_HOME, GOPROXY, node on PATH) that detached processes do not inherit.

  MUST be launched from a process OUTSIDE the Studio server's tree (WMI
  Win32_Process.Create or a one-shot scheduled task). The agent's own tool
  processes run as children of the Studio server; killing it from an in-tree
  process kills the agent's own turn and the restart never happens.

.PARAMETER StudioHome
  DSH home of the Studio profile. Default: $env:DSH_HOME, else <repo>\data\studio-home.

.PARAMETER Profile
  dsh profile name. Default: vivy-studio.

.PARAMETER Port
  Studio web port. Default: 3090.

.PARAMETER WaitSeconds
  Delay before killing, so the current agent turn settles and persists. Default: 15.

.PARAMETER DshCmd
  Path to dsh.cmd. Default: (Get-Command dsh) else C:\npmg\dsh.cmd.

.EXAMPLE
  powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File restart-studio.ps1 -WaitSeconds 15
#>
[CmdletBinding()]
param(
  [string]$StudioHome = "",
  [string]$Profile = "vivy-studio",
  [int]$Port = 3090,
  [int]$WaitSeconds = 15,
  [string]$DshCmd = ""
)

$ErrorActionPreference = "Stop"

# --- resolve repo root by walking up from this script's location ---
function Get-RepoRoot([string]$startDir) {
  $dir = $startDir
  for ($i = 0; $i -lt 8; $i++) {
    if (Test-Path (Join-Path $dir "launch-vivy-studio.ps1")) { return $dir }
    if (Test-Path (Join-Path $dir ".git")) { return $dir }
    $parent = Split-Path -Parent $dir
    if (-not $parent -or $parent -eq $dir) { return "" }
    $dir = $parent
  }
  return ""
}

$repoRoot = Get-RepoRoot $PSScriptRoot

# --- resolve StudioHome ---
if (-not $StudioHome) {
  if ($env:DSH_HOME) { $StudioHome = $env:DSH_HOME }
  elseif ($repoRoot) { $StudioHome = Join-Path $repoRoot "data\studio-home" }
  else { throw "cannot resolve StudioHome; pass -StudioHome explicitly" }
}
New-Item -ItemType Directory -Force -Path $StudioHome | Out-Null

$log = Join-Path $StudioHome "studio-restart.log"
function Log([string]$msg) {
  ("[{0}] {1}" -f (Get-Date -Format o), $msg) | Out-File -FilePath $log -Append -Encoding utf8
}

Log "restart-studio.ps1 begin (profile=$Profile port=$Port wait=$WaitSeconds)"
if ($repoRoot) { Log "repo root: $repoRoot" }

# --- find the listener on the Studio port ---
$conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
$listener = $null
if ($conn) { $listener = $conn.OwningProcess }

if ($listener) {
  # Safety: abort if this script is a descendant of the listener (in-tree spawn).
  $ancestors = @()
  $cur = $PID
  for ($i = 0; $i -lt 12 -and $cur -gt 0; $i++) {
    $ancestors += $cur
    $p = Get-CimInstance Win32_Process -Filter "ProcessId=$cur" -ErrorAction SilentlyContinue
    if (-not $p) { break }
    $cur = $p.ParentProcessId
  }
  if ($ancestors -contains $listener) {
    Log "ABORT: this script runs inside the Studio server's process tree (listener=$listener). Spawn it detached (WMI/schtasks) or the kill kills your own turn."
    exit 1
  }
  Log "listener pid=$listener; waiting $WaitSeconds s for the current turn to settle"
  Start-Sleep -Seconds $WaitSeconds
  Log "killing listener pid=$listener (taskkill /T /F)"
  & taskkill /PID $listener /T /F 2>&1 | ForEach-Object { Log ("taskkill: " + $_) }
} else {
  Log "no listener on port $Port; nothing to kill"
  Start-Sleep -Seconds 2
}

# --- wait for the port to free ---
$portFree = $false
for ($i = 0; $i -lt 30; $i++) {
  $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  if (-not $c) { $portFree = $true; break }
  Start-Sleep -Seconds 1
}
Log "port free: $portFree"

# --- resolve node + dsh for the boot environment ---
$nodeDir = ""
$nodePath = if ($listener) { (Get-Process -Id $listener -ErrorAction SilentlyContinue).Path } else { "" }
if ($nodePath) { $nodeDir = Split-Path -Parent $nodePath }
if (-not $nodeDir -and (Test-Path "C:\nvm4w\nodejs\node.exe")) { $nodeDir = "C:\nvm4w\nodejs" }
if (-not $nodeDir) {
  $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
  if ($nodeCmd) { $nodeDir = Split-Path -Parent $nodeCmd.Source }
}
if (-not $DshCmd) {
  $dshFound = Get-Command dsh -ErrorAction SilentlyContinue
  $DshCmd = if ($dshFound) { $dshFound.Source } elseif (Test-Path "C:\npmg\dsh.cmd") { "C:\npmg\dsh.cmd" } else { "dsh" }
}
$goBin = if (Test-Path "C:\Program Files\Go\bin") { "C:\Program Files\Go\bin" } else { "" }

# --- write the boot wrapper with an explicit environment ---
$boot = Join-Path $StudioHome "boot-studio.cmd"
$pathParts = @()
if ($goBin) { $pathParts += $goBin }
if ($repoRoot) { $pathParts += $repoRoot }
if ($nodeDir) { $pathParts += $nodeDir }
$pathLine = $pathParts -join ";"
$cdLine = ""
if ($repoRoot) { $cdLine = "cd /d `"$repoRoot`"" }
@"
@echo off
set "DSH_HOME=$StudioHome"
set "GOPROXY=https://goproxy.cn,direct"
set "PATH=$pathLine;%PATH%"
$cdLine
"$DshCmd" --profile $Profile --port $Port
"@ | Out-File -FilePath $boot -Encoding ascii

# --- relaunch detached and hidden ---
$stdout = Join-Path $StudioHome "studio-boot.stdout.log"
$stderr = Join-Path $StudioHome "studio-boot.stderr.log"
Log "launching: cmd /c $boot"
$p = Start-Process -FilePath "cmd.exe" -ArgumentList "/c", ("`"" + $boot + "`"") `
  -WorkingDirectory $StudioHome -WindowStyle Hidden `
  -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
Log "boot process started pid=$($p.Id)"

# --- wait for the port to come back ---
$up = $false
for ($i = 0; $i -lt 90; $i++) {
  Start-Sleep -Seconds 1
  $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  if ($c) { $up = $true; break }
  if ($p.HasExited) { Log "boot process exited early (code $($p.ExitCode))"; break }
}
Log "port up after restart: $up"
if (-not $up) {
  Log "stderr tail:"
  if (Test-Path $stderr) { Get-Content $stderr -Tail 20 | ForEach-Object { Log ("  " + $_) } }
  Log "stdout tail:"
  if (Test-Path $stdout) { Get-Content $stdout -Tail 20 | ForEach-Object { Log ("  " + $_) } }
}
Log "restart-studio.ps1 done"
