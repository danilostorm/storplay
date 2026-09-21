StorPlay Windows Development Package

PORTS
-----
Sunshine keeps its normal ports:
- 47989: GameStream HTTP
- 47990: Sunshine Web UI

StorPlay uses:
- 48120: StorPlay HTTP/WebSocket UI

NORMAL MODE
-----------
1. Extract the whole ZIP to a folder.
2. Start Sunshine first.
3. Verify Sunshine itself is running.
4. Double-click Start-StorPlay.bat.
5. Your browser opens http://localhost:48120.
6. Open http://localhost:48120/api/sunshine/info to verify detection.
7. Click Connect.

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
- Sunshine /serverinfo detection works when Sunshine is running.
- Actual Sunshine GameStream video/audio ingestion is the next integration.

Security note:
This development build is LAN-first and currently uses HTTP/WebSocket.
Do not expose port 48120 directly to the public Internet.
