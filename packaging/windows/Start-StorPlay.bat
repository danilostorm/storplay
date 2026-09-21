@echo off
setlocal
cd /d "%~dp0"

echo.
echo  StorPlay Host
echo  =============
echo.
echo  Opening http://localhost:47990
echo  Close this window to stop StorPlay.
echo.

start "" "http://localhost:47990"
storplay-host.exe -web-dir web

endlocal
