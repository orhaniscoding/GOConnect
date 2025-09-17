
param(
  [ValidateSet("agent", "controller")][string]$ServiceType = "agent",
  [string]$ExePath = "",
  [string]$ServiceName = "",
  [string]$DisplayName = ""
)

if ($ServiceType -eq "agent") {
  if (-not $ExePath) { $ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-service.exe" }
  if (-not $ServiceName) { $ServiceName = "GOConnectService" }
  if (-not $DisplayName) { $DisplayName = "GOConnect Agent Service" }
} elseif ($ServiceType -eq "controller") {
  if (-not $ExePath) { $ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnectcontroller.exe" }
  if (-not $ServiceName) { $ServiceName = "GOConnectController" }
  if (-not $DisplayName) { $DisplayName = "GOConnect Controller Service" }
} else {
  Write-Host "Unknown service type: $ServiceType"
  exit 1
}

Write-Host "Installing $ServiceType as Windows service..."
try {
    # Stop and uninstall in case it's already there
    & $ExePath stop | Out-Null
    & $ExePath uninstall | Out-Null

    # Install the service definition
    & $ExePath install

    if ($ServiceType -eq "agent") {
      # CRITICAL STEP: Run interactively once as the user to generate machine-level secrets.
      Write-Host "Pre-running agent to generate secrets..."
      $process = Start-Process -FilePath $ExePath -PassThru
      Start-Sleep -Seconds 5
      Stop-Process -Id $process.Id -Force
      Write-Host "Secrets generation step complete."
    }

    # Now, start the service, which will run as LocalSystem.
    & $ExePath start
    Write-Host "$ServiceType service installed and started."
} catch {
    Write-Host "An error occurred during service installation: $_"
    exit 1
}


