# Split-loop inner development: backend :8787 + Vite UI :3015.
# Ctrl+C stops both process trees. Open http://127.0.0.1:3015
# Usage: .\dev.ps1   or   just dev   or   .\dev.cmd

param(
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
Set-Location $root

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
        if ($Proc.HasExited) {
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

$script:subscribers = @()
$backend = $null
$ui = $null
try {
    if (Test-TcpPort 8787) {
        throw "127.0.0.1:8787 is already in use; stop the other Vivy process first"
    }
    if (Test-TcpPort 3015) {
        throw "127.0.0.1:3015 is already in use; stop the other Vite process first"
    }

    $uiDir = Join-Path $root "ui"
    if (-not (Test-Path (Join-Path $uiDir "node_modules"))) {
        Write-Host "installing ui dependencies"
        Push-Location $uiDir
        try {
            & $pnpm install
            if ($LASTEXITCODE) { throw "pnpm install failed: $LASTEXITCODE" }
        } finally {
            Pop-Location
        }
    }

    if (-not $env:VIVY_CONFIG -and -not $env:OPENAI_API_KEY -and -not $env:ANTHROPIC_API_KEY) {
        $env:VIVY_CONFIG = Join-Path $root "config.dev.yaml"
        Write-Host "no provider API key; using config.dev.yaml (runtime.mock=true)"
    }

    Write-Host "starting backend 127.0.0.1:8787"
    $backend = Start-LoggedProcess $go "run ./cmd/vivy" $root "vivy"
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
    Write-Host "stopping split loop"
    Stop-Tree $ui
    Stop-Tree $backend
    foreach ($sub in $script:subscribers) {
        Unregister-Event -SourceIdentifier $sub.Name -Force -ErrorAction SilentlyContinue
        Remove-Job -Name $sub.Name -Force -ErrorAction SilentlyContinue
    }
}
