# StorPlay

StorPlay is a self-hosted game streaming platform focused on **browser-first remote play** and **easy session sharing**.

The long-term goal is simple: install StorPlay on the gaming PC, open a browser on another device, and play with low latency — or create a temporary link so a friend can join without installing a VPN or native client.

> Status: early development / architecture phase.

## Goals

- Browser client with no installation required
- Low-latency WebRTC video/audio transport
- Keyboard, mouse and gamepad input
- Temporary share links for friends
- Viewer / gamepad-only / full-control permissions
- Direct P2P connection when possible, TURN relay when required
- Hardware video encoding (NVENC, AMF, Quick Sync / VA-API)
- Windows-first MVP, followed by Linux
- Sunshine/Moonlight interoperability during the transition
- Native StorPlay capture/encode engine as the long-term direction

## Initial architecture

```text
Gaming PC
┌───────────────────────────────────────────┐
│                 StorPlay Host             │
│                                           │
│  Sunshine/GameStream adapter (MVP)        │
│                 │                         │
│                 ▼                         │
│        encoded video + audio              │
│                 │                         │
│                 ▼                         │
│        WebRTC media transport             │
│                 │                         │
│  Input  ◄──── DataChannel                 │
└─────────────────┬─────────────────────────┘
                  │
             WebRTC / HTTPS
                  │
                  ▼
        Chrome / Edge / Firefox
```

The first milestone will use Sunshine as an optional streaming backend while StorPlay builds the browser/session layer. Later milestones will introduce a native capture and encoder pipeline so Sunshine is no longer required.

## MVP

The first usable target is:

**Windows host + Sunshine → StorPlay → browser → 1080p60 + audio + keyboard/mouse + Xbox-compatible gamepad**

Then:

**Share → generate temporary URL → friend opens URL → stream starts without Tailscale, ZeroTier or Moonlight.**

See [Architecture](docs/ARCHITECTURE.md) and [Roadmap](docs/ROADMAP.md).

## Inspiration and references

StorPlay studies established open-source game-streaming projects, including:

- [Sunshine](https://github.com/LizardByte/Sunshine)
- [Moonlight](https://github.com/moonlight-stream)
- [Moonlight-Web](https://github.com/linckosz/moonlight-web)

StorPlay is intended to have its own product architecture and UX. Reference projects may have different licenses; code reuse must follow the license of the reused component.

## Repository layout

```text
storplay/
├── host/       # native host and streaming adapters
├── web/        # browser player
├── server/     # signaling, invites and relay coordination
├── shared/     # protocol/schema shared by host and web
└── docs/       # architecture and roadmap
```

## Project principles

1. Browser-first, but never browser-only.
2. Avoid transcoding whenever the encoded stream can be forwarded directly.
3. Local streaming must work without a cloud dependency.
4. Internet access must be opt-in.
5. Guest links must be temporary, revocable and permission-scoped.
6. Security boundaries for remote input are treated as a core feature.

## License

License decision is intentionally pending while the project determines which upstream components, if any, will be incorporated directly. Sunshine and Moonlight-Web are GPL-licensed projects, so direct code reuse can affect StorPlay's licensing obligations.
