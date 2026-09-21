# StorPlay Roadmap

## M0 — Repository foundation

- [x] Define product goals
- [x] Define browser-first architecture
- [x] Separate host, media and control planes
- [x] Document Sunshine-backed MVP
- [ ] Choose final project license after dependency decisions
- [x] Set up CI

## M1 — Browser shell

- [x] TypeScript web client
- [x] WebRTC PeerConnection lifecycle
- [x] DataChannel connection
- [x] fullscreen player
- [x] keyboard input capture
- [x] pointer-lock mouse input
- [x] Gamepad API input
- [x] WebRTC statistics overlay
- [ ] validate against the first StorPlay host build on a real machine
- [ ] browser compatibility pass (Chrome/Edge/Firefox)

## M2 — Local host + signaling

- [x] StorPlay host executable foundation (Go + Pion WebRTC)
- [x] local HTTP health/info API
- [ ] local HTTPS control API
- [x] WebRTC offer/answer signaling
- [x] ICE candidate exchange
- [x] host-created low-latency input DataChannel
- [ ] Windows keyboard/mouse input injection
- [ ] Windows virtual gamepad input
- [ ] LAN session discovery
- [ ] authenticated admin UI

## M3 — Sunshine adapter

- [ ] Sunshine host discovery
- [ ] pairing/session bootstrap
- [ ] launch desktop/application
- [ ] receive video stream
- [ ] receive audio stream
- [ ] forward compatible encoded media into WebRTC
- [ ] translate browser input back to the host
- [ ] 1080p60 end-to-end test

## M4 — Sharing

- [ ] temporary invite links
- [ ] expiration
- [ ] optional PIN
- [ ] viewer permission
- [ ] gamepad-only permission
- [ ] full-control permission
- [ ] kick/revoke guest
- [ ] multiple guests

## M5 — Internet access

- [ ] STUN
- [ ] ICE direct connectivity
- [ ] TURN fallback
- [ ] public rendezvous/signaling service
- [ ] abuse/rate-limit controls
- [ ] NAT/CGNAT test matrix

## M6 — Native capture engine

- [ ] Windows capture backend
- [ ] NVENC
- [ ] AMD AMF
- [ ] Intel Quick Sync
- [ ] audio capture
- [ ] native input injection
- [ ] Sunshine-independent streaming

## M7 — Advanced

- [ ] AV1
- [ ] HDR
- [ ] high refresh rate
- [ ] virtual display
- [ ] WebCodecs rendering path
- [ ] WebGPU scaling/sharpening
- [ ] multi-controller local co-op
- [ ] Linux host
