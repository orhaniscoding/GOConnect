param(
    [string]$Version
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Resolve repo root
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$releasesDir = Join-Path $repoRoot 'releases'
$binDir = Join-Path $repoRoot 'bin'

# Ensure releases directory exists
New-Item -ItemType Directory -Force -Path $releasesDir | Out-Null

function Get-NextVersion([string]$releasesPath) {
    $dirs = Get-ChildItem -Path $releasesPath -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '^[0-9]+\.[0-9]+\.[0-9]+$' }
    if (-not $dirs -or $dirs.Count -eq 0) {
        return '0.0.1'
    }
    $versions = $dirs | ForEach-Object { [Version]$_.Name }
    $last = ($versions | Sort-Object -Descending | Select-Object -First 1)
    $next = [Version]::new($last.Major, $last.Minor, ($last.Build + 1))
    return $next.ToString()
}

if (-not $Version -or [string]::IsNullOrWhiteSpace($Version)) {
    $Version = Get-NextVersion -releasesPath $releasesDir
} else {
    if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+$') {
        throw "Version must be in SemVer format: MAJOR.MINOR.PATCH (e.g., 0.1.0)"
    }
}

$targetRoot = Join-Path $releasesDir $Version
$agentDir = Join-Path $targetRoot 'GOConnectAgentPackage'
$controllerDir = Join-Path $targetRoot 'GOConnectControllerPackage'

# Create target dirs
New-Item -ItemType Directory -Force -Path $agentDir | Out-Null
New-Item -ItemType Directory -Force -Path $controllerDir | Out-Null

# Validate binaries exist (build beforehand)
$svcExe = Join-Path $binDir 'goconnect-service.exe'
$ctlExe = Join-Path $binDir 'goconnectcontroller.exe'
if (-not (Test-Path $svcExe)) { throw "Missing $svcExe. Build the service binary first." }
if (-not (Test-Path $ctlExe)) { throw "Missing $ctlExe. Build the controller binary first." }

# Source scripts
$installAgent = Join-Path $repoRoot 'build\scripts\install-service-agent.ps1'
$uninstallAgent = Join-Path $repoRoot 'build\scripts\uninstall-service-agent.ps1'
$installCtl   = Join-Path $repoRoot 'build\scripts\install-service-controller.ps1'
$uninstallCtl = Join-Path $repoRoot 'build\scripts\uninstall-service-controller.ps1'

if (-not (Test-Path $installAgent)) { throw "Missing $installAgent" }
if (-not (Test-Path $uninstallAgent)) { throw "Missing $uninstallAgent" }
if (-not (Test-Path $installCtl)) { throw "Missing $installCtl" }
if (-not (Test-Path $uninstallCtl)) { throw "Missing $uninstallCtl" }

# Copy Agent package contents
Copy-Item -Force -Path $svcExe -Destination (Join-Path $agentDir 'goconnect-service.exe')
Copy-Item -Force -Path $installAgent -Destination (Join-Path $agentDir 'install-service-agent.ps1')
Copy-Item -Force -Path $uninstallAgent -Destination (Join-Path $agentDir 'uninstall-service-agent.ps1')

# Copy Controller package contents
Copy-Item -Force -Path $ctlExe -Destination (Join-Path $controllerDir 'goconnectcontroller.exe')
Copy-Item -Force -Path $installCtl -Destination (Join-Path $controllerDir 'install-service-controller.ps1')
Copy-Item -Force -Path $uninstallCtl -Destination (Join-Path $controllerDir 'uninstall-service-controller.ps1')

Write-Host "Release prepared: $targetRoot"