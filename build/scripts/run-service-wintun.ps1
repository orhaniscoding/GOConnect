param(
    [string]$UDPPort = "45820",
    [switch]$Verbose
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Run the GOConnect service with the wintun build tag for local development.
# Requires Windows and the Wintun driver/DLL.
# Tip: Run this PowerShell as Administrator if you want to actually bring up the TUN interface.

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
Push-Location $repoRoot
try {
    $env:GOFLAGS = ''
    if ($Verbose) { Write-Host "Running with -tags=wintun ..." -ForegroundColor Cyan }
    go run -tags=wintun ./cmd/goconnectservice
}
finally {
    Pop-Location
}
