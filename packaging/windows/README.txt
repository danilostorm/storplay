StorPlay Windows Development Package

1. Extract the whole ZIP to a folder.
2. Double-click Start-StorPlay.bat.
3. Your browser opens http://localhost:47990.
4. Click Connect in the StorPlay page.

Current milestone:
- WebRTC signaling works.
- ICE exchange works.
- The input DataChannel works.
- Keyboard/mouse/gamepad messages reach the host.
- Video/audio streaming is NOT connected yet.

The next milestone connects Sunshine/GameStream video and audio to the WebRTC session.

Security note:
This development build is LAN-first and currently uses HTTP/WebSocket.
Do not expose port 47990 directly to the public Internet.
