# Split-loop inner development: backend :8787 + Vite UI :3015.
# Ctrl+C stops both process trees. Open http://127.0.0.1:3015
# Usage: .\dev.ps1   or   just dev   or   .\dev.cmd
#        .\dev.ps1 -Split  -> backend and Vite in two separate windows
# A previous vivy backend on :8787 or Vite on :3015 is stopped automatically;
# ports held by unrelated processes are reported instead of killed.

param(
    [switch]$NoBrowser,
    [switch]$Split
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
Set-Location $root

& (Join-Path $root "scripts/ensure-laputa.ps1") -RepoRoot $root -Quiet

$go = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path $go)) {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        throw "go is not on PATH and was not found at C:\Program Files\Go\bin\go.exe"
    }
    $go = $goCmd.Source
}
$env:GOPROXY = "https://goproxy.cn,direct"
$goBin = Split-Path $go
if ($env:Path -notlike "*$goBin*") {
    $env:Path = "$goBin;$env:Path"
}

$pnpmCmd = Get-Command pnpm.cmd -ErrorAction SilentlyContinue
if (-not $pnpmCmd) {
    $pnpmCmd = Get-Command pnpm -ErrorAction SilentlyContinue
}
if (-not $pnpmCmd) {
    throw "pnpm is not on PATH; install Node.js 22+ and pnpm"
}
$pnpm = $pnpmCmd.Source

function Test-TcpPort([int]$Port) {
    try {
        $client = New-Object System.Net.Sockets.TcpClient
        $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(200)
        if (-not $ok) { $client.Close(); return $false }
        $client.EndConnect($iar) | Out-Null
        $client.Close()
        return $true
    } catch {
        return $false
    }
}

function Wait-TcpPort([int]$Port, [int]$Seconds, [string]$Label, $Proc) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        if ($Proc -and $Proc.HasExited) {
            throw "$Label exited before port $Port was ready (exit $($Proc.ExitCode))"
        }
        if (Test-TcpPort $Port) { return }
        Start-Sleep -Milliseconds 250
    }
    throw "$Label did not listen on 127.0.0.1:$Port within ${Seconds}s"
}

function Start-LoggedProcess([string]$File, [string]$Arguments, [string]$WorkDir, [string]$Prefix) {
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo.FileName = $File
    $p.StartInfo.Arguments = $Arguments
    $p.StartInfo.WorkingDirectory = $WorkDir
    $p.StartInfo.UseShellExecute = $false
    $p.StartInfo.RedirectStandardOutput = $true
    $p.StartInfo.RedirectStandardError = $true
    $p.StartInfo.RedirectStandardInput = $true
    $p.StartInfo.CreateNoWindow = $true
    $handler = {
        if (-not [string]::IsNullOrEmpty($EventArgs.Data)) {
            Write-Host ("[{0}] {1}" -f $Event.MessageData, $EventArgs.Data)
        }
    }
    $script:subscribers += Register-ObjectEvent -InputObject $p -EventName OutputDataReceived -Action $handler -MessageData $Prefix
    $script:subscribers += Register-ObjectEvent -InputObject $p -EventName ErrorDataReceived -Action $handler -MessageData $Prefix
    if (-not $p.Start()) {
        throw "failed to start $Prefix"
    }
    $p.BeginOutputReadLine()
    $p.BeginErrorReadLine()
    return $p
}

function Stop-Tree($Proc) {
    if ($null -eq $Proc) { return }
    if ($Proc.HasExited) { return }
    $procId = $Proc.Id
    & taskkill.exe /T /F /PID $procId 2>$null | Out-Null
}

# Return the PID of a dev.ps1-spawned window (identified by markers in its
# -Command line) that is an ancestor of $ProcId, so a restart kills the whole
# stale window instead of leaving it behind.
function Find-DevWindowAncestor([int]$ProcId, [string[]]$Markers) {
    $current = $ProcId
    for ($i = 0; $i -lt 5; $i++) {
        $wmi = Get-CimInstance Win32_Process -Filter "ProcessId=$current" -ErrorAction SilentlyContinue
        if (-not $wmi) { return $null }
        $parentId = $wmi.ParentProcessId
        $parentWmi = Get-CimInstance Win32_Process -Filter "ProcessId=$parentId" -ErrorAction SilentlyContinue
        if (-not $parentWmi) { return $null }
        if ($parentWmi.Name -match '^powershell' -and $Markers) {
            foreach ($marker in $Markers) {
                if ($parentWmi.CommandLine -and $parentWmi.CommandLine -like "*$marker*") {
                    return $parentId
                }
            }
        }
        $current = $parentId
    }
    return $null
}

# Stop a previous dev process that holds $Port, but only when it is really
# ours (process name, and for generic names like node, a command-line match).
function Stop-StaleDevProcess {
    param(
        [int]$Port,
        [string[]]$AllowedNames,
        [string]$Label,
        [string]$ListenerCmdMatch,
        [string[]]$WindowMarkers
    )
    if (-not (Test-TcpPort $Port)) { return }
    $owners = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique)
    foreach ($owner in $owners) {
        $proc = Get-Process -Id $owner -ErrorAction SilentlyContinue
        if (-not $proc) { continue }
        $ok = $AllowedNames -contains $proc.ProcessName
        if ($ok -and $ListenerCmdMatch) {
            $cmdLine = (Get-CimInstance Win32_Process -Filter "ProcessId=$owner" -ErrorAction SilentlyContinue).CommandLine
            $ok = $cmdLine -and ($cmdLine -like "*$ListenerCmdMatch*")
        }
        if (-not $ok) {
            throw "127.0.0.1:$Port is held by $($proc.ProcessName) (PID $owner), which is not a $Label process; stop it manually first"
        }
        $target = Find-DevWindowAncestor $owner $WindowMarkers
        if (-not $target) { $target = $owner }
        Write-Host "stopping previous $Label (PID $target)"
        & taskkill.exe /T /F /PID $target 2>$null | Out-Null
    }
    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline -and (Test-TcpPort $Port)) { Start-Sleep -Milliseconds 250 }
    if (Test-TcpPort $Port) {
        throw "127.0.0.1:$Port is still in use after stopping the previous process; stop it manually and retry"
    }
}

$script:subscribers = @()
$backend = $null
$ui = $null
try {
    Stop-StaleDevProcess -Port 8787 -AllowedNames @("vivy", "vivy-backend") -Label "vivy backend" -WindowMarkers @("vivy-backend")
    Stop-StaleDevProcess -Port 3015 -AllowedNames @("node") -Label "vite" -ListenerCmdMatch "vite" -WindowMarkers @("pnpm")

    $uiDir = Join-Path $root "ui"
    if (-not (Test-Path (Join-Path $uiDir "node_modules"))) {
        Write-Host "installing ui dependencies"
        Push-Location $uiDir
        try {
            & $pnpm install --frozen-lockfile
            if ($LASTEXITCODE) { throw "pnpm install failed: $LASTEXITCODE" }
        } finally {
            Pop-Location
        }
    }

    if (-not $env:VIVY_CONFIG -and -not $env:DEEPSEEK_API_KEY -and -not $env:OPENAI_API_KEY -and -not $env:ANTHROPIC_API_KEY) {
        Write-Host "no provider API key; Vivy will start without a model; configure Settings -> Model"
    }

    # Vite owns the development UI; a fresh clone has no embedded ui/dist.
    # Finish the cold compile before applying the server-readiness timeout.
    $buildDir = Join-Path $root ".workspace/dev"
    New-Item -ItemType Directory -Force -Path $buildDir | Out-Null
    $backendExe = Join-Path $buildDir "vivy-backend.exe"
    Write-Host "building split-loop backend"
    & $go build -tags vivy_headless -o $backendExe ./cmd/vivy
    if ($LASTEXITCODE) { throw "backend build failed: $LASTEXITCODE" }

    if ($Split) {
        # Two detached windows; the launcher exits after both ports answer.
        $backendCmd = "`$Host.UI.RawUI.WindowTitle = 'vivy backend :8787'; Set-Location -LiteralPath '$root'; & '$backendExe'"
        Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -NoExit -Command `"$backendCmd`"" | Out-Null
        $uiCmd = "`$Host.UI.RawUI.WindowTitle = 'vite :3015'; Set-Location -LiteralPath '$uiDir'; & '$pnpm' dev"
        Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -NoExit -Command `"$uiCmd`"" | Out-Null

        Wait-TcpPort 8787 90 "backend" $null
        Wait-TcpPort 3015 45 "vite" $null
        Write-Host ""
        Write-Host "split windows ready: http://127.0.0.1:3015  (control plane http://127.0.0.1:8787)"
        Write-Host "stop each part with Ctrl+C in its own window."
        if (-not $NoBrowser) {
            Start-Process "http://127.0.0.1:3015" | Out-Null
        }
        return
    }

    Write-Host "starting backend 127.0.0.1:8787"
    $backend = Start-LoggedProcess $backendExe "" $root "vivy"
    Wait-TcpPort 8787 90 "backend" $backend

    Write-Host "starting vite 127.0.0.1:3015"
    $ui = Start-LoggedProcess $pnpm "dev" $uiDir "vite"
    Wait-TcpPort 3015 45 "vite" $ui

    Write-Host ""
    Write-Host "split loop ready: http://127.0.0.1:3015  (control plane http://127.0.0.1:8787)"
    Write-Host "Ctrl+C stops both."
    if (-not $NoBrowser) {
        Start-Process "http://127.0.0.1:3015" | Out-Null
    }

    while (-not $backend.HasExited -and -not $ui.HasExited) {
        Start-Sleep -Seconds 1
    }
    if ($backend.HasExited) {
        throw "backend exited (code $($backend.ExitCode))"
    }
    if ($ui.HasExited) {
        throw "vite exited (code $($ui.ExitCode))"
    }
} finally {
    if ($backend -or $ui) {
        Write-Host "stopping split loop"
    }
    Stop-Tree $ui
    Stop-Tree $backend
    foreach ($sub in $script:subscribers) {
        Unregister-Event -SourceIdentifier $sub.Name -Force -ErrorAction SilentlyContinue
        Remove-Job -Name $sub.Name -Force -ErrorAction SilentlyContinue
    }
}
