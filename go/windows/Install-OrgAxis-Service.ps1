#requires -Version 5.1
#requires -RunAsAdministrator
param(
  [string]$LANHost = '',
  [int]$LANPort = 8766,
  [switch]$ImportExistingDemoData
)
$ErrorActionPreference = 'Stop'
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourceExe = Join-Path $scriptDir 'OrgAxis.exe'
if (-not (Test-Path $sourceExe)) { throw "Missing OrgAxis.exe in $scriptDir" }
$base = Join-Path $env:ProgramData 'OrgAxis'
$binDir = Join-Path $base 'bin'
$dataDir = Join-Path $base 'data'
New-Item -ItemType Directory -Force -Path $binDir,$dataDir | Out-Null
# SID S-1-5-19 = NT AUTHORITY\LocalService, not LocalSystem.
& icacls.exe $dataDir /grant '*S-1-5-19:(OI)(CI)M' /T | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot grant LocalService access to service data directory' }
$existing = Get-Service -Name 'OrgAxis' -ErrorAction SilentlyContinue
if ($existing) { throw 'OrgAxis Windows Service already exists. Stop and uninstall it before installing again.' }
$targetExe = Join-Path $binDir 'OrgAxis.exe'
Copy-Item -LiteralPath $sourceExe -Destination $targetExe -Force
if ($ImportExistingDemoData) {
  $oldData = Join-Path $scriptDir 'data'
  $oldDB = Join-Path $oldData 'orgaxis.sqlite3'
  $newDB = Join-Path $dataDir 'orgaxis.sqlite3'
  if (Test-Path $newDB) { throw 'Service database already exists. Import skipped to protect it.' }
  if (-not (Test-Path $oldDB)) { throw "Local demo database not found: $oldDB" }
  if (Test-Path "$oldDB-wal") { throw 'SQLite WAL exists: stop all OrgAxis processes and checkpoint the database before importing.' }
  if (Test-Path "$oldDB-shm") { throw 'SQLite SHM exists: verify the old process is stopped, then use a consistent SQLite backup.' }
  Copy-Item -LiteralPath $oldDB -Destination $newDB
}
if ($LANHost) {
  $address = $null
  if (-not [System.Net.IPAddress]::TryParse($LANHost,[ref]$address)) { throw 'LANHost must be an IP, e.g. 192.168.11.11' }
}
$binPath = '"' + $targetExe + '" -mode service -no-browser -port 8765 -data "' + $dataDir + '"'
if ($LANHost) { $binPath += ' -lan-host ' + $LANHost + ' -lan-port ' + $LANPort }
& sc.exe create 'OrgAxis' binPath= $binPath start= auto obj= 'NT AUTHORITY\LocalService' DisplayName= 'OrgAxis Workforce Demo'
if ($LASTEXITCODE -ne 0) { throw 'sc.exe create failed' }
& sc.exe description 'OrgAxis' 'Temporal workforce engine (synthetic-data demonstration)'
& sc.exe failure 'OrgAxis' reset= 86400 actions= restart/5000/restart/15000/restart/30000 | Out-Null
if ($LANHost) {
  # Explicit LAN opt-in: only the local subnet can reach the read-only HTTPS endpoint.
  New-NetFirewallRule -Name 'OrgAxis-Demo-LAN' -DisplayName 'OrgAxis read-only TLS demo' -Enabled True -Direction Inbound -Action Allow -Protocol TCP -LocalPort $LANPort -RemoteAddress LocalSubnet -Profile Domain,Private -Program $targetExe | Out-Null
}
Start-Service -Name 'OrgAxis'
Write-Host "OrgAxis service installed and started. Data: $dataDir"
Write-Host 'Local URL: http://127.0.0.1:8765/'
Write-Host 'Use OrgAxis.exe -mode client for a tray icon attached to this Windows Service.'
if ($LANHost) { Write-Host "LAN TLS read-only URL: https://${LANHost}:$LANPort/ (requires token from $dataDir\lan\api-token.txt and certificate verification)" }
