# Compatibility entry; the portable Node script is the sole bootstrap owner.
[CmdletBinding()]
param([string]$RepoRoot, [switch]$Quiet)
$ErrorActionPreference = 'Stop'
$bootstrapArgs = @((Join-Path $PSScriptRoot 'ensure-laputa.mjs'))
if ($RepoRoot) { $bootstrapArgs += @('--repo-root', $RepoRoot) }
if ($Quiet) { $bootstrapArgs += '--quiet' }
& node @bootstrapArgs
exit $LASTEXITCODE
