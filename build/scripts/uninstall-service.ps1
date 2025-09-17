$ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-service.exe"
Write-Host "Uninstalling GOConnect service..."
& $ExePath stop
& $ExePath uninstall
Write-Host "Service uninstalled."

