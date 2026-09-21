StorPlay Windows Development Package

NORMAL MODE
-----------
1. Extract the whole ZIP to a folder.
2. Make sure Sunshine is running on this PC if you want /api/sunshine/info to detect it.
3. Double-click Start-StorPlay.bat.
4. Your browser opens http://localhost:47990.
5. Click Connect.

H264 DIAGNOSTIC MODE
--------------------
Double-click Start-StorPlay-Diagnostic.bat and then click Connect.

Expected result:
- WebRTC connects.
- The browser receives a real H.264 RTP video track.
- A static blue 320x180 frame appears.

If the blue frame appears, the StorPlay browser media path is working.
This diagnostic does NOT use Sunshine's actual game video.

Current milestone:
- WebRTC signaling works.
- ICE exchange works.
- H.264 + Opus media tracks are negotiated.
- H.264 diagnostic video can be pushed through the real media relay.
- The input DataChannel works.
- Keyboard/mouse/gamepad messages reach the host.
- Sunshine /serverinfo detection works.
- Actual Sunshine GameStream video/audio ingestion is the next integration.

Security note:
This development build is LAN-first and currently uses HTTP/WebSocket.
Do not expose port 47990 directly to the public Internet.
