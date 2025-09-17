# Uninstall GOConnect Agent Service
# Usage: Run this script as Administrator

$ServiceName = "GOConnectService"

if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    Write-Host "Stopping $ServiceName..."
    Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host "Removing $ServiceName..."
    sc.exe delete $ServiceName | Out-Null
    Write-Host "Service removed."
} else {
    Write-Host "Service '$ServiceName' not found."
}