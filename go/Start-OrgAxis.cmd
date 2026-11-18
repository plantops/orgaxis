@echo off
cd /d "%~dp0"
OrgAxis-Windows10-x64.exe -port 6666
if errorlevel 1 pause
