# Install GOConnect Agent as a Windows Service
# Usage: Run this script as Administrator

$ServiceName = "GOConnect"
$ServiceDisplayName = "GOConnect Agent Service"
$ServiceDescription = "GOConnect VPN Agent."
$ExePath = Join-Path $PSScriptRoot "goconnect-service.exe"

if (-not (Test-Path $ExePath)) {
    Write-Error "Agent binary not found: $ExePath"
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