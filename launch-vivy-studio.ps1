$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$plugin = Join-Path $PSScriptRoot "studio\dsh-vivy-studio"
$console = Join-Path $PSScriptRoot "studio\dsh-vivy-console"
$pluginHub = Join-Path $PSScriptRoot "studio\dsh-plugin-hub"
$homeDir = Join-Path $root "data\studio-home"
$profileDir = Join-Path $homeDir "profiles\vivy-studio"
$storageDir = Join-Path $homeDir "storages"
New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
New-Item -ItemType Directory -Force -Path $storageDir | Out-Null

$env:DSH_HOME = $homeDir
$env:GOPROXY = "https://goproxy.cn,direct"
$goBin = "C:\Program Files\Go\bin"
if (Test-Path $goBin) { $env:Path = "$goBin;$env:Path" }
$env:Path = "$root;$env:Path"
Set-Location $root

$dsh = "C:\npmg\dsh.cmd"
if (-not (Test-Path $dsh)) { $dsh = "dsh" }

$sdk = Join-Path $root "vivy-sdk.exe"
if (-not (Test-Path $sdk)) {
  Write-Host "building vivy-sdk.exe"
  if (Get-Command just -ErrorAction SilentlyContinue) {
    & just sdk
  } else {
    & go build -o vivy-sdk.exe ./sdk
  }
}

$wsId = "a1e2c3d4-5b67-4890-abcd-ef0123456789"
$now = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
$workspaceDoc = [ordered]@{
  unit = [ordered]@{ name = "workspace"; version = 2 }
  global = [ordered]@{
    initialized = $true
    workspaceIds = @($wsId)
    archivedSessionIds = @()
  }
  tables = [ordered]@{
    workspaces = [ordered]@{
      $wsId = [ordered]@{
        path = $root
        title = "agent-vivy"
        sessionIds = @()
        createdAt = $now
        updatedAt = $now
      }
    }
  }
}
# Write UTF-8 without BOM: dsh JSON.parse chokes on a BOM and PS 5.1
# Set-Content -Encoding utf8 always adds one.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $storageDir "workspace.json"), ($workspaceDoc | ConvertTo-Json -Depth 6), $utf8NoBom)

$pluginUnix = ($plugin -replace "\\", "/")
$consoleUnix = ($console -replace "\\", "/")
$pluginHubUnix = ($pluginHub -replace "\\", "/")
$profilePkg = Join-Path $profileDir "package.json"
$vivyRoot = $root
$env:VIVY_ROOT = $vivyRoot

# Build the profile manifest from the canonical first-party bundles, then merge
# any Vivy-source plugins installed by dsh-plugin-hub so they survive restarts.
$deps = [ordered]@{
  "dsh-vivy-studio"  = "file:$pluginUnix"
  "dsh-vivy-console" = "file:$consoleUnix"
  "dsh-plugin"       = "file:$pluginHubUnix"
}
$bundles = [System.Collections.Generic.List[string]]::new()
@("@deepseek-ai/dsh-base", "@deepseek-ai/dsh-web-app", "dsh-vivy-studio", "dsh-vivy-console", "dsh-plugin") | ForEach-Object { $bundles.Add($_) }

$registryPath = Join-Path $profileDir "vivy-source-plugins.json"
if (Test-Path $registryPath) {
  try {
    $registry = Get-Content -Raw -Path $registryPath | ConvertFrom-Json -ErrorAction Stop
    foreach ($entry in $registry.plugins) {
      $localPath = ($entry.localPath -replace "\\", "/")
      if (-not [System.IO.Path]::IsPathRooted($localPath)) {
        $localPath = Join-Path $vivyRoot $localPath
      }
      $localPath = ($localPath -replace "\\", "/")
      $deps[$entry.name] = "file:$localPath"
      if (-not $bundles.Contains($entry.name)) { $bundles.Add($entry.name) }
    }
  } catch {
    Write-Host "warning: failed to merge vivy-source-plugins.json: $_"
  }
}

$manifest = [ordered]@{
  name         = "dsh-profile-vivy-studio"
  private      = $true
  dependencies = $deps
  dsh          = [ordered]@{
    profile = [ordered]@{
      bundles = $bundles
    }
  }
}

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$manifestJson = $manifest | ConvertTo-Json -Depth 6
[System.IO.File]::WriteAllText($profilePkg, $manifestJson, $utf8NoBom)

$needSeal = $true
if (Test-Path $profilePkg) {
  $profileText = Get-Content -Raw -Path $profilePkg
  $needSeal = ($profileText -notmatch '"dsh-vivy-studio"') -or ($profileText -notmatch '"dsh-vivy-console"') -or ($profileText -notmatch '"dsh-plugin"')
}

if ($needSeal) {
  Write-Host "linking first-party skin + console + plugin-hub"
  & $dsh plugin --profile vivy-studio add "file:$plugin" "file:$console" "file:$pluginHub"
} else {
  # Ensure any merged vivy-source file: dependencies are materialized.
  & pnpm install --dir $profileDir | Out-Null
}

# The retired dsh-vivy-debugger bundle is no longer composed; drop any stale
# installed copy so the floating-button plugin cannot linger.
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue (Join-Path $profileDir "node_modules\dsh-vivy-debugger")

Write-Host "DSH_HOME=$env:DSH_HOME"
Write-Host "workspace=$root"
Write-Host "profile=vivy-studio"
Write-Host "open http://127.0.0.1:3090"

& $dsh --profile vivy-studio --port 3090
