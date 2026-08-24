$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$plugin = Join-Path $PSScriptRoot "studio\dsh-vivy-studio"
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
$profilePkg = Join-Path $profileDir "package.json"
$needSeal = $true
if (Test-Path $profilePkg) {
  $needSeal = (Get-Content -Raw -Path $profilePkg) -notmatch "dsh-vivy-studio"
}

# Official web-app resolves from the dsh install, not npm. Do not
# `dsh plugin add @deepseek-ai/dsh-web-app` — that 404s on dsh-frontend.
@"
{
  "name": "dsh-profile-vivy-studio",
  "private": true,
  "dependencies": {
    "dsh-vivy-studio": "file:$pluginUnix"
  },
  "dsh": {
    "profile": {
      "bundles": [
        "@deepseek-ai/dsh-base",
        "@deepseek-ai/dsh-web-app",
        "dsh-vivy-studio"
      ]
    }
  }
}
"@ | ForEach-Object { [System.IO.File]::WriteAllText($profilePkg, $_, $utf8NoBom) }

if ($needSeal) {
  Write-Host "linking first-party skin"
  & $dsh plugin --profile vivy-studio add "file:$plugin"
}

Write-Host "DSH_HOME=$env:DSH_HOME"
Write-Host "workspace=$root"
Write-Host "profile=vivy-studio"
Write-Host "open http://127.0.0.1:3090"

& $dsh --profile vivy-studio --port 3090
