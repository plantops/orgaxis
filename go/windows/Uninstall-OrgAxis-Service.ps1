#requires -Version 5.1
#requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$svc=Get-Service -Name OrgAxis -ErrorAction SilentlyContinue
if ($svc) {
  if ($svc.Status -ne 'Stopped') { Stop-Service -Name OrgAxis -Force; $svc.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(25)) }
  & sc.exe delete 'OrgAxis'
  if ($LASTEXITCODE -ne 0) { throw 'sc.exe delete failed' }
}
Get-NetFirewallRule -Name 'OrgAxis-Demo-LAN' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Write-Host 'OrgAxis service uninstalled. The database in %ProgramData%\OrgAxis\data is retained.'
