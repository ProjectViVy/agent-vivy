$ErrorActionPreference = 'Stop'
$utf8 = [System.Text.UTF8Encoding]::new($false)
[Console]::InputEncoding = $utf8
[Console]::OutputEncoding = $utf8
$OutputEncoding = $utf8

try {
    $repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
    Set-Location -LiteralPath $repoRoot

    # start.cmd looks in CARGO_TARGET_DIR first, then .generated\target, target
    # and C:\vt. Build where it will look: the set directory, else the one the
    # builder uses by default.
    if (-not $env:CARGO_TARGET_DIR) {
        $env:CARGO_TARGET_DIR = Join-Path $repoRoot '.generated\target'
    }
    $targetDirectory = [System.IO.Path]::GetFullPath($env:CARGO_TARGET_DIR).TrimEnd('\')
    $env:CARGO_TARGET_DIR = $targetDirectory

    # Debug information is not needed to run the agent, and the shorter links
    # keep MSVC below its path limits in native dependencies.
    if (-not $env:CARGO_PROFILE_DEV_DEBUG) {
        $env:CARGO_PROFILE_DEV_DEBUG = '0'
    }

    $recipe = if ($args.Count -ge 1) { $args[0] } else { 'recipes/coding-agent.yaml' }
    if (-not (Test-Path -LiteralPath $recipe -PathType Leaf)) {
        throw "The recipe was not found: $recipe"
    }

    # Windows cannot replace a running executable: the link would fail after a
    # long compile. Name the window to close instead.
    $running = @(Get-Process -Name 'vivy', 'coding-agent' -ErrorAction SilentlyContinue |
        Where-Object {
            $path = try { $_.Path } catch { $null }
            $path -and $path.StartsWith("$targetDirectory\", [System.StringComparison]::OrdinalIgnoreCase)
        })
    if ($running.Count -gt 0) {
        $names = ($running | ForEach-Object { "$($_.ProcessName) (PID $($_.Id))" }) -join ', '
        throw "This build is in use: $names. Close that terminal window, then run build.cmd again."
    }

    Write-Host "Building $recipe into $targetDirectory"
    Write-Host 'The first build takes minutes; a source change rebuilds only what it touches.'
    & cargo run -p vivy -- build $recipe
    if ($LASTEXITCODE -ne 0) {
        throw "The build failed with code $LASTEXITCODE; its output above names the cause."
    }

    Write-Host "Built. start.cmd runs this build from $targetDirectory." -ForegroundColor Green
    $exitCode = 0
} catch {
    Write-Host "Build failed: $($_.Exception.Message)" -ForegroundColor Red
    $exitCode = 1
}

Read-Host 'Press Enter to close'
exit $exitCode
