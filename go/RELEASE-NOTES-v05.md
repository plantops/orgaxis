# OrgAxis v0.5

- Windows tray UI runs without CMD, supports open workspace/logs and HKCU Start-with-Windows toggle.
- Single-instance guard protects the database across Windows logon sessions; separate tray UI guard.
- Windows Service mode under LocalService with installer/uninstaller scripts; independent login.
- Optional read-only LAN API over TLS with separate 256-bit bearer token and private-IP binding.
- Diagnostics page and local API, log rollover at 10 MiB with four backups.
- Local API at 127.0.0.1:8765; all original HR data and workflows retained.

Validation: Go unit tests and -race pass on Linux, HTTP smoke test passes, HTTPS LAN smoke test passes, GUI and console Windows x64 cross-build succeeds. Native Windows 10 tray and SCM validation outstanding. Not for real employee data.
