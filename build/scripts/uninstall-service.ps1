
$ServiceName = "GOConnect"
$ExePath = Join-Path $PSScriptRoot "..\..\bin\goconnect-service.exe"
Write-Host "Uninstalling GOConnect service..."

# Servisi durdur
if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
	try {
		Stop-Service -Name $ServiceName -Force -ErrorAction Stop
		Write-Host "Service stopped."
	} catch {
		Write-Host "Service could not be stopped or was not running."
	}
}

# Servisi kaldır
& $ExePath uninstall

# Kaldırıldı mı kontrol et, hala varsa registry'den sil
Start-Sleep -Seconds 2
if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
	Write-Host "Service still exists, attempting registry cleanup..."
	$svcKey = "HKLM:\\SYSTEM\\CurrentControlSet\\Services\\$ServiceName"
	Remove-Item -Path $svcKey -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Service uninstalled."

