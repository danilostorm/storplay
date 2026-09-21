# StorPlay Architecture

## 1. Product model

StorPlay is split into three logical planes:

- **Host plane** — runs on the gaming PC and owns capture, encode and input injection.
- **Media plane** — transports video/audio/input with low latency.
- **Control plane** — handles discovery, authentication, signaling, invites and session permissions.

The browser should never need raw TCP/UDP access. Browser clients communicate using HTTPS/WebSocket for control and WebRTC for real-time media/input.

## 2. Phase 1: Sunshine-backed MVP

The fastest route to a working browser stream is to avoid rebuilding capture and hardware encoding immediately.

```text
Game / Desktop
      │
      ▼
   Sunshine
      │
      │ GameStream-compatible session
      ▼
StorPlay Host Adapter
      │
      ├── video/audio packet adaptation
      ├── input translation
      └── session lifecycle
      │
      ▼
    WebRTC
      │
      ▼
 Browser Client
```

A key target is **no unnecessary decode/re-encode**. When browser codec support and the source stream are compatible, StorPlay should repacketize/forward encoded frames rather than transcode them.

## 3. Phase 2: Native StorPlay host

After the browser and session system are stable:

```text
DXGI / Windows Graphics Capture
            │
            ▼
      GPU surface capture
            │
            ▼
 NVENC / AMF / Quick Sync
            │
            ▼
       WebRTC sender
            │
            ▼
          Browser
```

This removes the Sunshine dependency and makes StorPlay a complete host.

## 4. Browser client

Initial web-client components:

- WebRTC PeerConnection
- video element for the first MVP
- WebCodecs path for advanced rendering later
- Pointer Lock API for gaming mouse mode
- Gamepad API for controllers
- keyboard event translation
- fullscreen / wake lock
- stats overlay: RTT, jitter, packet loss, bitrate, decode FPS
- adaptive bitrate / resolution controls

WebGPU-based scaling/sharpening is a later enhancement, not an MVP dependency.

## 5. Input transport

Input should travel over a low-latency WebRTC DataChannel.

Logical messages:

```text
InputMessage
├── keyboard
├── mouse_move
├── mouse_button
├── mouse_wheel
├── gamepad_state
├── gamepad_button
└── control_command
```

The first version can use compact JSON during development. Before performance tuning, migrate high-frequency messages such as mouse/gamepad state to a binary schema.

## 6. Session sharing

A session invite has:

- opaque random token
- host/session ID
- expiration
- permission scope
- optional PIN
- maximum guests
- revoked flag

Permission scopes:

- `viewer`
- `gamepad`
- `full-control`

Never expose a permanent host credential in a share URL.

## 7. Internet connectivity

Preferred connection order:

1. direct LAN
2. direct WebRTC ICE/P2P
3. TURN relay

The control/signaling server should not be required for LAN-only streaming after session establishment.

## 8. Security requirements

Remote input is equivalent to remote desktop access and must be protected accordingly.

Required from the first public Internet-capable release:

- TLS
- DTLS/SRTP through WebRTC
- cryptographically random invite tokens
- short-lived guest sessions
- host-side explicit Internet opt-in
- permission-scoped guest input
- immediate revoke/kick
- rate limiting
- CSRF and origin validation for host management APIs
- no arbitrary command execution through game/app metadata

## 9. Technology direction

### Host

Initial candidate:

- C++20
- CMake
- libdatachannel or equivalent WebRTC stack
- adapter layer for Sunshine/GameStream
- platform-specific input injection

The native engine can later use platform APIs directly for capture and hardware encode.

### Web

- TypeScript
- lightweight build tooling
- browser-native WebRTC APIs
- progressive use of WebCodecs/WebGPU

### Signaling

Keep signaling protocol implementation-independent. It may initially run inside the host for LAN use and optionally through a small public rendezvous service for Internet sharing.

## 10. Upstream projects

Moonlight-Web demonstrates several useful architecture patterns:

- native host + browser frontend separation
- WebRTC media transport
- DataChannel input
- native capture/encode path
- virtual display support
- session-sharing concepts
- compatibility with Sunshine/GameStream hosts

StorPlay should treat these as architectural references rather than blindly mirroring the repository structure.
