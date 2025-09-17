param(
  [string]$ExePath = "",
  [string]$ServiceName = "GOConnectService",
  [string]$DisplayName = "GOConnect Service"
)

$ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-service.exe"
Write-Host "Installing GOConnect service..."
try {
    # Stop and uninstall in case it's already there
    & "$PSScriptRoot\..\..\bin\goconnect-service.exe" stop | Out-Null
    & "$PSScriptRoot\..\..\bin\goconnect-service.exe" uninstall | Out-Null
    
    # Install the service definition
    & "$PSScriptRoot\..\..\bin\goconnect-service.exe" install
    
    # CRITICAL STEP: Run interactively once as the user to generate machine-level secrets.
    # The DPAPI calls are configured to use machine-level protection, so secrets
    # created by the user in this step will be readable by the LocalSystem service.
    Write-Host "Pre-running service to generate secrets..."
    $process = Start-Process -FilePath "$PSScriptRoot\..\..\bin\goconnect-service.exe" -PassThru
    # Give it a moment to run the startup and key generation logic.
    Start-Sleep -Seconds 5 
    Stop-Process -Id $process.Id -Force
    Write-Host "Secrets generation step complete."

    # Now, start the service, which will run as LocalSystem.
    & "$PSScriptRoot\..\..\bin\goconnect-service.exe" start
    Write-Host "Service installed and started."
} catch {
    Write-Host "An error occurred during service installation: $_"
    exit 1
}


