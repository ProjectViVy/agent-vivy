param(
    [Parameter(Mandatory=$true)][string]$Root,
    [string]$Python = 'python',
    [switch]$Managed
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
foreach ($name in @('UV_CONFIG_FILE', 'UV_PROJECT', 'UV_WORKING_DIR', 'UV_SYSTEM_PYTHON')) {
    [Environment]::SetEnvironmentVariable($name, $null, 'Process')
}
$rootPath = [IO.Path]::GetFullPath($Root)
[IO.Directory]::CreateDirectory($rootPath) | Out-Null
$lock = $null
try {
    $lock = [IO.File]::Open((Join-Path $rootPath 'bootstrap.lock'), 'OpenOrCreate', 'ReadWrite', 'None')
    $env:UV_PYTHON_INSTALL_DIR = Join-Path $rootPath 'python'
    $env:UV_PYTHON_BIN_DIR = Join-Path $rootPath 'bin'
    $env:UV_CACHE_DIR = Join-Path $rootPath 'cache'
    $env:UV_NO_MODIFY_PATH = '1'
    $env:UV_PYTHON_DOWNLOADS = 'manual'
    $env:UV_NO_CONFIG = '1'
    $uv = Join-Path $rootPath 'uv-0.11.7/uv.exe'
    $existing = Get-Command uv -ErrorAction SilentlyContinue
    if ($existing -and -not $Managed) {
        $version = & $existing.Source --version
        if ($LASTEXITCODE -eq 0 -and $version -match '^uv 0\.11\.7( |$)') { $uv = $existing.Source }
    }
    if (-not (Test-Path -LiteralPath $uv)) {
        if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'Managed bootstrap currently supports Windows x64 only.' }
        @{type='stage';stage='download-uv'} | ConvertTo-Json -Compress
        $candidate = Join-Path $rootPath ([Guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($candidate) | Out-Null
        $zip = Join-Path $candidate 'uv.zip'
        Invoke-WebRequest -UseBasicParsing -TimeoutSec 120 -Uri 'https://github.com/astral-sh/uv/releases/download/0.11.7/uv-x86_64-pc-windows-msvc.zip' -OutFile $zip
        if ((Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant() -ne 'fe0c7815acf4fc45f8a5eff58ed3cf7ae2e15c3cf1dceadbd10c816ec1690cc1') {
            throw 'uv archive SHA256 verification failed; nothing was installed.'
        }
        Expand-Archive -LiteralPath $zip -DestinationPath $candidate
        $binary = Join-Path $candidate 'uv.exe'
        if (-not (Test-Path -LiteralPath $binary)) { throw 'Verified uv archive has no executable.' }
        [IO.Directory]::CreateDirectory((Split-Path $uv)) | Out-Null
        Move-Item -LiteralPath $binary -Destination $uv
    }
    $interpreter = $null
    if (-not $Managed) {
        try {
            $found = & $Python -I -c 'import sys; assert sys.version_info >= (3,11); print(sys.executable)' 2>$null
            if ($LASTEXITCODE -eq 0) { $interpreter = ($found | Select-Object -Last 1).Trim() }
        } catch { }
        if (-not $interpreter -and $Python -ne 'python') {
            throw 'Configured Python could not be started. Choose Use managed Python in /setup python, or correct python.executable.'
        }
    }
    if (-not $interpreter) {
        @{type='stage';stage='install-python'} | ConvertTo-Json -Compress
        & $uv python install --no-bin --no-registry --install-dir $env:UV_PYTHON_INSTALL_DIR 3.13.12
        if ($LASTEXITCODE -ne 0) { throw 'Managed Python 3.13.12 installation failed.' }
        $found = & $uv python find --managed-python --no-python-downloads 3.13.12
        if ($LASTEXITCODE -ne 0) { throw 'Managed Python executable was not found after installation.' }
        $interpreter = ($found | Select-Object -Last 1).Trim()
    }
    @{type='toolchain';python=$interpreter;uv=$uv} | ConvertTo-Json -Compress
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
} finally {
    if ($lock) { $lock.Dispose() }
}
