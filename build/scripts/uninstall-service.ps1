param(
  [string]$ServiceName = "GOConnectService"
)

Write-Host "Stopping and removing service '$ServiceName'"
sc.exe stop $ServiceName | Out-Null
Start-Sleep -Seconds 1
sc.exe delete $ServiceName | Out-Null
Write-Host "Service removed."

