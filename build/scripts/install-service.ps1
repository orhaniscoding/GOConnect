param(
  [string]$ExePath = "",
  [string]$ServiceName = "GOConnectService",
  [string]$DisplayName = "GOConnect Service"
)

if (-not $ExePath) {
  $ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-service.exe"
}

Write-Host "Installing service '$ServiceName' with binary: $ExePath"

# Example using sc.exe. In production, you may prefer NSSM or kardianos/service installer.
sc.exe create $ServiceName binPath= '"' + $ExePath + '"' start= delayed-auto DisplayName= '"' + $DisplayName + '"' | Out-Null
sc.exe description $ServiceName "GOConnect agent service with local API and web UI" | Out-Null
sc.exe start $ServiceName | Out-Null
Write-Host "Service installed and started."

