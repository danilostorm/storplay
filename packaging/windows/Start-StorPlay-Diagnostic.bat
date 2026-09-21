@echo off
setlocal
cd /d "%~dp0"

echo.
echo  StorPlay H264 Diagnostic
echo  ========================
echo.
echo  This test does not use Sunshine video.
echo  It proves StorPlay Host -> WebRTC -> browser H264 playback.
echo.
echo  Opening http://localhost:47990
echo  Click Connect. You should see a blue video frame.
echo.

start "" "http://localhost:47990"
storplay-host.exe -web-dir web -test-pattern

endlocal
