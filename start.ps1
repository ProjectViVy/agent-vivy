$ErrorActionPreference = 'Stop'
$utf8 = [System.Text.UTF8Encoding]::new($false)
[Console]::InputEncoding = $utf8
[Console]::OutputEncoding = $utf8
$OutputEncoding = $utf8

try {
    $repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
    Set-Location -LiteralPath $repoRoot

    # A build can be in CARGO_TARGET_DIR, in the checkout's own target, or in
    # .generated\target when CARGO_TARGET_DIR was not set. Prefer the one
    # directory that holds both executables: a launcher from one build and a
    # species from another can disagree about the recipe.
    $roots = @()
    if ($env:CARGO_TARGET_DIR) { $roots += $env:CARGO_TARGET_DIR }
    $roots += @('.generated\target', 'target', 'C:\vt')
    $builds = @($roots | Where-Object { $_ } | ForEach-Object {
        $root = if ([System.IO.Path]::IsPathRooted($_)) { $_ } else { Join-Path $repoRoot $_ }
        Join-Path ([System.IO.Path]::GetFullPath($root).TrimEnd('\')) 'debug'
    } | Select-Object -Unique)

    $built = $builds | Where-Object {
        (Test-Path -LiteralPath (Join-Path $_ 'vivy.exe') -PathType Leaf) -and
        (Test-Path -LiteralPath (Join-Path $_ 'coding-agent.exe') -PathType Leaf)
    } | Select-Object -First 1

    if ($built) {
        $agent = Join-Path $built 'vivy.exe'
        $targetDirectory = Split-Path -Parent $built
    } else {
        $agent = $builds | ForEach-Object { Join-Path $_ 'vivy.exe' } |
            Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
        $species = $builds | ForEach-Object { Join-Path $_ 'coding-agent.exe' } |
            Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
        if (-not $agent -or -not $species) {
            $searched = ($builds | ForEach-Object { "  $_" }) -join "`n"
            throw "The coding agent is not built yet. Build it once from this folder:`n  `$env:CARGO_TARGET_DIR = 'C:\vt'; `$env:CARGO_PROFILE_DEV_DEBUG = '0'; cargo run -p vivy -- build recipes/coding-agent.yaml`nLooked for vivy.exe and coding-agent.exe in:`n$searched"
        }
        $targetDirectory = Split-Path -Parent (Split-Path -Parent $species)
    }

    $env:CARGO_TARGET_DIR = $targetDirectory
    $desktop = [Environment]::GetFolderPath('Desktop')
    $credentialFile = @(
        (Join-Path $desktop 'keys.txt'),
        (Join-Path $desktop 'key.txt')
    ) | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1

    if ($credentialFile) {
        $values = @([System.IO.File]::ReadAllLines($credentialFile, [System.Text.Encoding]::UTF8) |
            ForEach-Object { $_.Trim() } |
            Where-Object { $_.Length -gt 0 })
        if ($values.Count -lt 3 -or $values.Count -gt 4) {
            throw 'The Desktop key file must have three or four non-empty lines: gateway URL, model ID, API key, and optionally the Rig dialect (such as `deepseek`).'
        }

        $endpoint = $null
        if (-not [Uri]::TryCreate($values[0], [UriKind]::Absolute, [ref]$endpoint) -or
            $endpoint.Scheme -notin @('https', 'http')) {
            throw 'The gateway URL in the Desktop key file is invalid.'
        }
        if ($endpoint.Scheme -eq 'http' -and
            $endpoint.Host -notin @('localhost', '127.0.0.1', '::1')) {
            throw 'Use HTTPS for a non-local gateway so the API key is not sent in clear text.'
        }

        $rigHome = Join-Path $repoRoot '.workspace\launch-home'
        $env:RIG_HOME = $rigHome
        $env:VIVY_BOOTSTRAP_GATEWAY_URL = $values[0]
        $env:VIVY_BOOTSTRAP_GATEWAY_MODEL = $values[1]
        $env:VIVY_BOOTSTRAP_GATEWAY_DIALECT = if ($values.Count -eq 4) { $values[3] } else { $null }
        $env:RIG_SECRET_GATEWAY = $values[2]
        Write-Host 'Loaded the Desktop gateway hint; existing provider settings take priority and the API key stays in process memory.'
    } else {
        if (-not $env:RIG_HOME) {
            $env:RIG_HOME = Join-Path $HOME '.rig'
        }
        # An empty home is usable: /provider adds the first provider and key.
    }

    Write-Host "Starting the coding terminal in $repoRoot"
    & $agent
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        Write-Host "The coding terminal exited with code $exitCode." -ForegroundColor Yellow
    }
} catch {
    Write-Host "Startup failed: $($_.Exception.Message)" -ForegroundColor Red
    $exitCode = 1
}

Read-Host 'Press Enter to close'
exit $exitCode
