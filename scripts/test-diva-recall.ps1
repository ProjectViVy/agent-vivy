# Exercise opt-in native recall through the official SDK-generated DIVA recipe.
[CmdletBinding()]
param([string]$RepoRoot)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) { $RepoRoot = Split-Path -Parent $PSScriptRoot }
$root = (Resolve-Path -LiteralPath $RepoRoot).Path
$scratch = Join-Path ([System.IO.Path]::GetTempPath()) ("vivy-recall-" + [guid]::NewGuid().ToString("N"))
$tests = @(
    "TestMemoryLoopMemoryInjection",
    "TestMemoryLoopRecallAfterProcessRestart",
    "TestMemoryLoopRecallNegativeControls",
    "TestMemoryLoopOrdinaryRecallAuthorityBoundary",
    "TestMemoryLoopRecallDeadlineDegradesSafely",
    "TestMemoryLoopCorrectionAndDeletionInModelInput"
)
New-Item -ItemType Directory -Path $scratch | Out-Null
Push-Location $root
try {
    $generated = Join-Path $scratch "zz_diva.go"
    & go run ./sdk/internal/cmd/generate-default --repo $root --recipe recipes/diva.vivy.yml --output $generated
    if ($LASTEXITCODE) { throw "DIVA assembly generation failed ($LASTEXITCODE)" }
    $overlay = Join-Path $scratch "overlay.json"
    $replace = @{}
    $replace[(Join-Path $root "internal/generated/assembly/zz_default.go")] = $generated
    # Windows PowerShell's UTF8 Set-Content adds a BOM, which Go rejects.
    [System.IO.File]::WriteAllText($overlay, (@{ Replace = $replace } | ConvertTo-Json -Depth 3), [System.Text.UTF8Encoding]::new($false))
    $results = Join-Path $scratch "results.jsonl"
    $selection = "^(" + ($tests -join "|") + ")$"
    & go test -timeout 10m -overlay $overlay -tags vivy_diva_integration -json ./internal/app -run $selection -count=1 > $results
    $testExit = $LASTEXITCODE
    if ($testExit) {
        Get-Content -LiteralPath $results | Write-Output
        throw "DIVA recall tests failed ($testExit)"
    }
    $events = @(Get-Content -LiteralPath $results | ForEach-Object { $_ | ConvertFrom-Json })
    if (@($events | Where-Object { $_.Action -eq "skip" }).Count) {
        throw "DIVA recall gate must execute every selected test without skips"
    }
    foreach ($test in $tests) {
        if (@($events | Where-Object { $_.Test -eq $test -and $_.Action -eq "pass" }).Count -ne 1) {
            throw "DIVA recall gate did not pass required test $test"
        }
        Write-Output "PASS $test (SDK-generated DIVA assembly)"
    }
} finally {
    Pop-Location
    Remove-Item -LiteralPath $scratch -Recurse -Force
}
