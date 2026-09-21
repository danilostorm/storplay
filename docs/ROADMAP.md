# StorPlay Roadmap

## M0 — Repository foundation

- [x] Define product goals
- [x] Define browser-first architecture
- [x] Separate host, media and control planes
- [x] Document Sunshine-backed MVP
- [ ] Choose final project license after dependency decisions
- [ ] Set up CI

## M1 — Browser shell

- [ ] TypeScript web client
- [ ] WebRTC PeerConnection lifecycle
- [ ] DataChannel connection
- [ ] fullscreen player
- [ ] keyboard input capture
- [ ] pointer-lock mouse input
- [ ] Gamepad API input
- [ ] WebRTC statistics overlay

## M2 — Local host + signaling

- [ ] StorPlay host service for Windows
- [ ] local HTTPS control API
- [ ] WebRTC offer/answer signaling
- [ ] ICE candidate exchange
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
