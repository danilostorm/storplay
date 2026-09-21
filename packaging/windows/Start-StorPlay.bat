@echo off
setlocal
cd /d "%~dp0"

echo.
echo  StorPlay Host
echo  =============
echo.
echo  Sunshine keeps its default ports (47989/47990).
echo  StorPlay uses http://localhost:48120
echo  Close this window to stop StorPlay.
echo.

start "" "http://localhost:48120"
storplay-host.exe -web-dir web

endlocal
