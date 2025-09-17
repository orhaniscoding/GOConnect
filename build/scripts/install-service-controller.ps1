# Install GOConnect Controller as a Windows Service
# Usage: Run this script as Administrator

$ServiceName = "GOConnectController"
$ServiceDisplayName = "GOConnect Controller Service"
$ServiceDescription = "GOConnect VPN Controller."
$ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-controller.exe"

if (-not (Test-Path $ExePath)) {
    Write-Error "Controller binary not found: $ExePath"
    exit 1
}

if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    Write-Host "Service '$ServiceName' already exists. Removing first..."
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 2
}

Write-Host "Installing $ServiceDisplayName..."
sc.exe create $ServiceName binPath= "`"$ExePath`"" DisplayName= "$ServiceDisplayName" start= auto
sc.exe description $ServiceName "$ServiceDescription"

Start-Sleep -Seconds 1

Write-Host "Starting $ServiceDisplayName..."
Start-Service $ServiceName

Write-Host "Service installed and started successfully."