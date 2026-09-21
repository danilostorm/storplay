# StorPlay Host

The first native StorPlay host is written in Go and uses Pion WebRTC.

## What works in this milestone

- HTTP health/info endpoints
- WebSocket signaling
- WebRTC offer/answer
- trickle ICE
- low-latency unordered input DataChannel
- browser keyboard/mouse/gamepad protocol parsing
- explicit input-sink boundary for future OS input injection
- optional static serving of the built web client
- Sunshine `/serverinfo` probing and XML parsing

Video/audio are intentionally not wired yet. The next host milestone uses the Sunshine/GameStream session as the media source for WebRTC tracks.

## Run

```bash
cd host
go mod tidy
go run ./cmd/storplay-host
```

By default StorPlay probes a Sunshine instance on:

```text
http://127.0.0.1:47989
```

You can point it at another host:

```bash
go run ./cmd/storplay-host -sunshine http://192.168.1.50:47989
```

Or disable Sunshine probing:

```bash
go run ./cmd/storplay-host -sunshine ""
```

Useful local endpoints:

```text
GET /healthz
GET /api/info
GET /api/sunshine/info
WS  /ws/session
```

Then, in another terminal:

```bash
cd web
npm install
npm run dev
```

Open the URL printed by Vite. The browser defaults to:

```text
ws://<same-host>:47990/ws/session
```

When the WebRTC connection becomes connected, keyboard/mouse/gamepad input is sent to the host. For now the default sink only logs low-frequency input.

### Serve a production web build from the host

```bash
cd web
npm run build

cd ../host
go run ./cmd/storplay-host -web-dir ../web/dist
```

Then open:

```text
http://localhost:47990
```

## Internet mode

The current milestone is LAN-first. A STUN server can already be supplied with:

```bash
go run ./cmd/storplay-host -stun stun:your-stun-server:3478
```

TURN, authentication, guest links and public rendezvous are later milestones.
