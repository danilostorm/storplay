import type { ClientSignal, HostSignal, InputMessage } from './protocol';

export interface SessionStats {
  rttMs: number | null;
  bitrateMbps: number | null;
  fps: number | null;
  packetsLost: number | null;
}

type SessionState = 'disconnected' | 'connecting' | 'connected' | 'failed';

export class WebRtcSession {
  private pc: RTCPeerConnection | null = null;
  private ws: WebSocket | null = null;
  private input: RTCDataChannel | null = null;
  private statsTimer: number | null = null;
  private previousBytes = 0;
  private previousStatsAt = 0;

  onStateChange?: (state: SessionState) => void;
  onStats?: (stats: SessionStats) => void;
  onError?: (error: Error) => void;

  constructor(
    private readonly signalingUrl: string,
    private readonly videoElement: HTMLVideoElement,
  ) {}

  async connect(): Promise<void> {
    if (this.pc || this.ws) {
      throw new Error('Session is already active');
    }

    this.onStateChange?.('connecting');

    const pc = new RTCPeerConnection();
    this.pc = pc;

    pc.ondatachannel = (event) => {
      if (event.channel.label !== 'input') {
        event.channel.close();
        return;
      }

      this.input?.close();
      this.input = event.channel;

      this.input.onclose = () => {
        if (this.input === event.channel) this.input = null;
      };
    };

    pc.ontrack = (event) => {
      const [stream] = event.streams;
      if (stream) {
        this.videoElement.srcObject = stream;
      } else {
        const fallback = this.videoElement.srcObject instanceof MediaStream
          ? this.videoElement.srcObject
          : new MediaStream();
        fallback.addTrack(event.track);
        this.videoElement.srcObject = fallback;
      }
      void this.videoElement.play().catch(() => undefined);
    };

    pc.onicecandidate = (event) => {
      if (!event.candidate) return;
      this.sendSignal({
        type: 'ice',
        candidate: event.candidate.toJSON(),
      });
    };

    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'connected') {
        this.onStateChange?.('connected');
        this.startStats();
      } else if (pc.connectionState === 'failed') {
        this.onStateChange?.('failed');
      } else if (pc.connectionState === 'closed' || pc.connectionState === 'disconnected') {
        this.onStateChange?.('disconnected');
      }
    };

    await this.openSignaling();
  }

  sendInput(message: InputMessage): boolean {
    const channel = this.input;
    if (!channel || channel.readyState !== 'open') return false;

    channel.send(JSON.stringify(message));
    return true;
  }

  async close(): Promise<void> {
    this.stopStats();

    if (this.input?.readyState === 'open') {
      this.sendInput({ type: 'release_all' });
    }

    this.input?.close();
    this.input = null;

    this.ws?.close();
    this.ws = null;

    this.pc?.close();
    this.pc = null;

    this.videoElement.srcObject = null;
    this.onStateChange?.('disconnected');
  }

  private openSignaling(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.signalingUrl);
      this.ws = ws;

      ws.onopen = () => resolve();

      ws.onerror = () => {
        const error = new Error('Unable to open StorPlay signaling WebSocket');
        this.onError?.(error);
        reject(error);
      };

      ws.onmessage = (event) => {
        void this.handleSignal(event.data);
      };

      ws.onclose = () => {
        if (this.pc?.connectionState !== 'connected') {
          this.onStateChange?.('disconnected');
        }
      };
    });
  }

  private async handleSignal(raw: unknown): Promise<void> {
    if (typeof raw !== 'string' || !this.pc) return;

    let message: HostSignal;
    try {
      message = JSON.parse(raw) as HostSignal;
    } catch {
      this.onError?.(new Error('Invalid signaling message'));
      return;
    }

    try {
      switch (message.type) {
        case 'offer': {
          await this.pc.setRemoteDescription(message.sdp);
          const answer = await this.pc.createAnswer();
          await this.pc.setLocalDescription(answer);
          this.sendSignal({ type: 'answer', sdp: answer });
          break;
        }
        case 'ice':
          await this.pc.addIceCandidate(message.candidate);
          break;
        case 'error':
          this.onError?.(new Error(message.message));
          break;
        case 'session-ended':
          await this.close();
          break;
      }
    } catch (error) {
      this.onError?.(error instanceof Error ? error : new Error(String(error)));
    }
  }

  private sendSignal(message: ClientSignal): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify(message));
  }

  private startStats(): void {
    this.stopStats();
    this.previousBytes = 0;
    this.previousStatsAt = performance.now();

    this.statsTimer = window.setInterval(() => {
      void this.collectStats();
    }, 1000);
  }

  private stopStats(): void {
    if (this.statsTimer !== null) {
      window.clearInterval(this.statsTimer);
      this.statsTimer = null;
    }
  }

  private async collectStats(): Promise<void> {
    if (!this.pc) return;

    const reports = await this.pc.getStats();
    let rttMs: number | null = null;
    let bitrateMbps: number | null = null;
    let fps: number | null = null;
    let packetsLost: number | null = null;
    let videoBytes = 0;

    reports.forEach((report) => {
      if (report.type === 'candidate-pair' && report.state === 'succeeded' && report.currentRoundTripTime) {
        rttMs = report.currentRoundTripTime * 1000;
      }

      if (report.type === 'inbound-rtp' && report.kind === 'video') {
        videoBytes += report.bytesReceived ?? 0;
        fps = report.framesPerSecond ?? fps;
        packetsLost = report.packetsLost ?? packetsLost;
      }
    });

    const now = performance.now();
    const deltaMs = now - this.previousStatsAt;
    if (this.previousBytes > 0 && deltaMs > 0 && videoBytes >= this.previousBytes) {
      bitrateMbps = ((videoBytes - this.previousBytes) * 8) / (deltaMs / 1000) / 1_000_000;
    }

    this.previousBytes = videoBytes;
    this.previousStatsAt = now;

    this.onStats?.({ rttMs, bitrateMbps, fps, packetsLost });
  }
}
