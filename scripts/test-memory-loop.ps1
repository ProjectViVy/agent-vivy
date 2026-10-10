# Run the memory-loop suite against the Recipe it exercises, using the SDK's
# existing packed-artifact identity check and generated Go overlay.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$scratch = Join-Path $root (".workspace/memory-loop-ci-" + [guid]::NewGuid().ToString("N"))
$artifact = Join-Path $scratch "artifact"
$overlay = Join-Path $scratch "overlay"
$savedArtifact = $env:VIVY_MEMORY_LOOP_ARTIFACT
$savedOverlay = $env:VIVY_MEMORY_LOOP_OVERLAY_DIR

function Invoke-Go([string[]]$Arguments) {
    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Memory-loop verification failed: go $($Arguments -join ' ') (exit $LASTEXITCODE)"
    }
}

Push-Location $root
try {
    # The default pass excludes this prefix globally. Cover its ordinary
    # runtime/SDK/module tests before the App tests requiring the DIVA body.
    $packages = & go list ./...
    if ($LASTEXITCODE -ne 0) { throw "Could not list Go packages" }
    $packages = @($packages | Where-Object { $_ -ne "agent-vivy/internal/app" })
    Invoke-Go (@("test", "-timeout", "35m", "-run", "^TestMemoryLoop") + $packages)
    New-Item -ItemType Directory -Force $scratch | Out-Null
    Invoke-Go @("run", "./sdk", "pack", "--recipe", "recipes/diva.vivy.yml", "--output", $artifact)
    Invoke-Go @("run", "./sdk", "inspect-artifact", $artifact)
    $env:VIVY_MEMORY_LOOP_ARTIFACT = $artifact
    $env:VIVY_MEMORY_LOOP_OVERLAY_DIR = $overlay
    Invoke-Go @("test", "./sdk/internal", "-run", "^TestMemoryLoop", "-count=1")
    $env:VIVY_MEMORY_LOOP_ARTIFACT = $savedArtifact
    $env:VIVY_MEMORY_LOOP_OVERLAY_DIR = $savedOverlay
    Push-Location (Join-Path $root "internal/app")
    try {
        $package = & go list -json . | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw "Could not list App test sources" }
        # These files assert fields belonging only to the default body;
        # they run in the first pass and contain no TestMemoryLoop tests.
        $defaultBodyTests = @("default_generation_test.go", "headless_mask_arming_test.go", "assembly_notebook_test.go")
        foreach ($file in $defaultBodyTests) {
            if (Select-String -LiteralPath $file -Pattern '^func TestMemoryLoop' -Quiet) {
                throw "Default-body exclusion would omit memory-loop tests: $file"
            }
        }
        $tests = @($package.TestGoFiles | Where-Object { $_ -notin $defaultBodyTests })
        $sources = @($package.GoFiles) + $tests
        Invoke-Go (@("test", "-overlay", (Join-Path $overlay "overlay.json"), "-timeout", "35m", "-run", "^TestMemoryLoop") + $sources)
    } finally { Pop-Location }
} finally {
    $env:VIVY_MEMORY_LOOP_ARTIFACT = $savedArtifact
    $env:VIVY_MEMORY_LOOP_OVERLAY_DIR = $savedOverlay
    Remove-Item -LiteralPath $scratch -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
