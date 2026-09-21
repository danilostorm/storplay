# StorPlay Web

Early browser client for StorPlay.

## Current scope

- WebRTC session lifecycle
- WebSocket signaling skeleton
- native browser video rendering
- unreliable/low-latency input DataChannel
- keyboard input
- pointer-lock mouse input
- Gamepad API polling
- basic WebRTC stats

This is a protocol scaffold, not a working game stream yet. A StorPlay host/signaling implementation still needs to provide the SDP offer, ICE candidates and media tracks.

## Development

```bash
npm install
npm run dev
```

The signaling URL is editable in the UI while the host protocol is under development.
