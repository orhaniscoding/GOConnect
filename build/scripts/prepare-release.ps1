param(
    [Parameter(Mandatory=$true)]
    [string]$Version
)

$ErrorActionPreference = "Stop"

# Normalize version (strip leading v if present)
if ($Version.StartsWith("v")) { $Version = $Version.Substring(1) }

$Root = (Resolve-Path ".").Path
$Bin  = Join-Path $Root "bin"

# Ensure binaries exist (build if missing) with ldflags
$svcExe = Join-Path $Bin "goconnect-service.exe"
$ctlExe = Join-Path $Bin "goconnectcontroller.exe"

if (!(Test-Path $svcExe)) {
    & go build -ldflags "-X goconnect/internal/version.Version=$Version" -o $svcExe ./cmd/goconnectservice
}
if (!(Test-Path $ctlExe)) {
    & go build -ldflags "-X goconnect/internal/version.Version=$Version" -o $ctlExe ./cmd/goconnectcontroller
}

# Prepare package folders
$agentDir = "GOConnectAgentPackage"
$ctrlDir  = "GOConnectControllerPackage"

if (Test-Path $agentDir) { Remove-Item $agentDir -Recurse -Force }
if (Test-Path $ctrlDir)  { Remove-Item $ctrlDir  -Recurse -Force }

New-Item -ItemType Directory -Path $agentDir | Out-Null
New-Item -ItemType Directory -Path $ctrlDir  | Out-Null

# Copy binaries + (placeholder) scripts; keep if real scripts exist
function Ensure-File($path, $content) {
    if (!(Test-Path $path)) { New-Item -ItemType File -Path $path -Force | Out-Null; Set-Content -Path $path -Value $content -Encoding UTF8 }
}
Ensure-File "build\scripts\install-service-agent.ps1"       'Write-Host "Install agent (placeholder)";'
Ensure-File "build\scripts\uninstall-service-agent.ps1"     'Write-Host "Uninstall agent (placeholder)";'
Ensure-File "build\scripts\install-service-controller.ps1"  'Write-Host "Install controller (placeholder)";'
Ensure-File "build\scripts\uninstall-service-controller.ps1"'Write-Host "Uninstall controller (placeholder)";'

Copy-Item $svcExe "$agentDir\goconnect-service.exe" -Force
Copy-Item $ctlExe "$ctrlDir\goconnectcontroller.exe" -Force
Copy-Item "build\scripts\install-service-agent.ps1"      $agentDir -Force
Copy-Item "build\scripts\uninstall-service-agent.ps1"    $agentDir -Force
Copy-Item "build\scripts\install-service-controller.ps1" $ctrlDir  -Force
Copy-Item "build\scripts\uninstall-service-controller.ps1" $ctrlDir -Force

# ZIP names
$agentZip = "GOConnectAgentPackage-$Version.zip"
$ctrlZip  = "GOConnectControllerPackage-$Version.zip"

# Create ZIPs
if (Test-Path $agentZip) { Remove-Item $agentZip -Force }
if (Test-Path $ctrlZip)  { Remove-Item $ctrlZip  -Force }
Compress-Archive -Path ".\$agentDir\*" -DestinationPath ".\$agentZip" -Force
Compress-Archive -Path ".\$ctrlDir\*"  -DestinationPath ".\$ctrlZip"  -Force

# SHA256 checksums
(Get-FileHash ".\$agentZip" -Algorithm SHA256).Hash | Out-File ".\$agentZip.sha256" -Encoding ascii
(Get-FileHash ".\$ctrlZip"  -Algorithm SHA256).Hash | Out-File ".\$ctrlZip.sha256"  -Encoding ascii

Write-Host "Release artifacts ready:"
Write-Host "  $agentZip (+ .sha256)"
Write-Host "  $ctrlZip  (+ .sha256)"