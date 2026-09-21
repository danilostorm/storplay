import { InputController } from './input';
import { WebRtcSession } from './webrtc';

type PairingState = 'unpaired' | 'waiting_for_pin' | 'pairing' | 'paired' | 'error';

interface SunshineInfoResponse {
  available: boolean;
  server?: {
    hostname: string;
    appVersion: string;
    pairStatus: number;
    state: string;
    currentGame: number;
  };
  pairing?: {
    state: PairingState;
    pin?: string;
    error?: string;
  };
  error?: string;
}

interface SunshineApp {
  id: number;
  title: string;
  hdrSupported: boolean;
}

interface LaunchSession {
  appId: number;
  sessionUrl: string;
  width: number;
  height: number;
  fps: number;
  hdr: boolean;
  startedAt: string;
}

const video = document.querySelector<HTMLVideoElement>('#player')!;
const status = document.querySelector<HTMLElement>('#status')!;
const connectButton = document.querySelector<HTMLButtonElement>('#connect')!;
const fullscreenButton = document.querySelector<HTMLButtonElement>('#fullscreen')!;
const mouseLockButton = document.querySelector<HTMLButtonElement>('#mouse-lock')!;
const signalingInput = document.querySelector<HTMLInputElement>('#signaling-url')!;
const emptyState = document.querySelector<HTMLElement>('#empty-state')!;

const sunshineSummary = document.querySelector<HTMLElement>('#sunshine-summary')!;
const sunshineRefresh = document.querySelector<HTMLButtonElement>('#sunshine-refresh')!;
const sunshinePair = document.querySelector<HTMLButtonElement>('#sunshine-pair')!;
const pairHelp = document.querySelector<HTMLElement>('#pair-help')!;
const pairPin = document.querySelector<HTMLElement>('#pair-pin')!;
const pairError = document.querySelector<HTMLElement>('#pair-error')!;
const appsPanel = document.querySelector<HTMLElement>('#apps-panel')!;
const appsList = document.querySelector<HTMLElement>('#apps-list')!;

const launchPanel = document.querySelector<HTMLElement>('#launch-panel')!;
const launchTitle = document.querySelector<HTMLElement>('#launch-title')!;
const launchSummary = document.querySelector<HTMLElement>('#launch-summary')!;
const launchURL = document.querySelector<HTMLElement>('#launch-url')!;
const launchCancel = document.querySelector<HTMLButtonElement>('#launch-cancel')!;

const rtt = document.querySelector<HTMLElement>('#rtt')!;
const bitrate = document.querySelector<HTMLElement>('#bitrate')!;
const fps = document.querySelector<HTMLElement>('#fps')!;
const loss = document.querySelector<HTMLElement>('#loss')!;

const signalScheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
const signalHost =
  window.location.port === '48120'
    ? window.location.host
    : `${window.location.hostname || 'localhost'}:48120`;
signalingInput.value = `${signalScheme}://${signalHost}/ws/session`;

let session: WebRtcSession | null = null;
let inputController: InputController | null = null;
let pairingTimer: number | null = null;
let launching = false;

function apiURL(path: string): string {
  if (window.location.port === '48120') return path;
  return `http://${window.location.hostname || 'localhost'}:48120${path}`;
}

async function refreshSunshine(): Promise<void> {
  sunshineRefresh.disabled = true;
  try {
    const response = await fetch(apiURL('/api/sunshine/info'));
    const data = (await response.json()) as SunshineInfoResponse;
    if (!response.ok || !data.available || !data.server) {
      sunshineSummary.textContent = data.error ?? 'Sunshine is not available.';
      sunshinePair.disabled = true;
      return;
    }

    sunshineSummary.textContent =
      `${data.server.hostname} · Sunshine ${data.server.appVersion} · ${data.server.state}`;

    const state = data.pairing?.state ?? (data.server.pairStatus === 1 ? 'paired' : 'unpaired');
    renderPairingState(state, data.pairing?.pin, data.pairing?.error);

    if (state === 'paired') {
      await loadApps();
      await loadActiveSession();
    }
  } catch (error) {
    sunshineSummary.textContent = `Unable to query Sunshine: ${String(error)}`;
    sunshinePair.disabled = true;
  } finally {
    sunshineRefresh.disabled = false;
  }
}

function renderPairingState(state: PairingState, pin?: string, error?: string): void {
  pairHelp.hidden = true;
  pairError.hidden = true;

  if (state === 'paired') {
    sunshinePair.disabled = true;
    sunshinePair.textContent = 'Paired';
    return;
  }

  if (state === 'waiting_for_pin' || state === 'pairing') {
    sunshinePair.disabled = true;
    sunshinePair.textContent = state === 'pairing' ? 'Pairing…' : 'Waiting for PIN…';
    if (pin) {
      pairPin.textContent = pin;
      pairHelp.hidden = false;
    }
    return;
  }

  if (state === 'error') {
    sunshinePair.disabled = false;
    sunshinePair.textContent = 'Try pairing again';
    pairError.textContent = error ?? 'Pairing failed.';
    pairError.hidden = false;
    return;
  }

  sunshinePair.disabled = false;
  sunshinePair.textContent = 'Pair';
}

async function startPairing(): Promise<void> {
  stopPairingPoll();
  appsPanel.hidden = true;
  pairError.hidden = true;
  sunshinePair.disabled = true;

  try {
    const response = await fetch(apiURL('/api/sunshine/pair/start'), { method: 'POST' });
    const data = (await response.json()) as { state: PairingState; pin?: string; error?: string };
    if (!response.ok) throw new Error(data.error ?? 'Unable to start pairing');

    renderPairingState(data.state, data.pin, data.error);
    pollPairing();
  } catch (error) {
    renderPairingState('error', undefined, String(error));
  }
}

function pollPairing(): void {
  const tick = async () => {
    try {
      const response = await fetch(apiURL('/api/sunshine/pair/status'));
      const data = (await response.json()) as { state: PairingState; pin?: string; error?: string };
      renderPairingState(data.state, data.pin, data.error);

      if (data.state === 'paired') {
        stopPairingPoll();
        await loadApps();
        await refreshSunshine();
        return;
      }
      if (data.state === 'error') {
        stopPairingPoll();
        return;
      }
    } catch (error) {
      renderPairingState('error', undefined, String(error));
      stopPairingPoll();
      return;
    }

    pairingTimer = window.setTimeout(tick, 1500);
  };

  pairingTimer = window.setTimeout(tick, 1000);
}

function stopPairingPoll(): void {
  if (pairingTimer !== null) {
    window.clearTimeout(pairingTimer);
    pairingTimer = null;
  }
}

async function loadApps(): Promise<void> {
  try {
    const response = await fetch(apiURL('/api/sunshine/apps'));
    const data = (await response.json()) as { apps?: SunshineApp[]; error?: string };
    if (!response.ok || !data.apps) {
      throw new Error(data.error ?? 'Unable to read Sunshine apps');
    }

    appsList.replaceChildren();
    for (const app of data.apps) {
      const button = document.createElement('button');
      button.className = 'app-chip app-launch';
      button.type = 'button';
      button.textContent = app.hdrSupported ? `${app.title} · HDR` : app.title;
      button.title = `Launch ${app.title}`;
      button.disabled = launching;
      button.addEventListener('click', () => {
        void launchApp(app, button);
      });
      appsList.appendChild(button);
    }
    appsPanel.hidden = false;
  } catch (error) {
    pairError.textContent = String(error);
    pairError.hidden = false;
  }
}

async function launchApp(app: SunshineApp, button: HTMLButtonElement): Promise<void> {
  if (launching) return;
  launching = true;
  pairError.hidden = true;

  const oldLabel = button.textContent;
  button.textContent = `Launching ${app.title}…`;
  for (const el of appsList.querySelectorAll<HTMLButtonElement>('button')) {
    el.disabled = true;
  }

  try {
    const response = await fetch(apiURL('/api/sunshine/launch'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        appId: app.id,
        width: 1920,
        height: 1080,
        fps: 60,
        bitrateKbps: 20000,
        hdr: false,
        playHostAudio: false,
      }),
    });

    const data = (await response.json()) as { session?: LaunchSession; error?: string };
    if (!response.ok || !data.session) {
      throw new Error(data.error ?? 'Sunshine launch failed');
    }

    renderLaunchSession(data.session, app.title);
    await refreshSunshine();
  } catch (error) {
    pairError.textContent = String(error);
    pairError.hidden = false;
  } finally {
    launching = false;
    button.textContent = oldLabel;
    for (const el of appsList.querySelectorAll<HTMLButtonElement>('button')) {
      el.disabled = false;
    }
  }
}

async function loadActiveSession(): Promise<void> {
  try {
    const response = await fetch(apiURL('/api/sunshine/session'));
    const data = (await response.json()) as { session: LaunchSession | null };
    if (!response.ok || !data.session) {
      if (response.ok) launchPanel.hidden = true;
      return;
    }
    renderLaunchSession(data.session);
  } catch {
    // Session display is diagnostic-only; Sunshine info remains the source of truth.
  }
}

function renderLaunchSession(current: LaunchSession, appTitle?: string): void {
  launchTitle.textContent = appTitle ? `${appTitle} launched` : `App ${current.appId} launched`;
  launchSummary.textContent =
    `${current.width}×${current.height} @ ${current.fps} FPS · GameStream session created successfully.`;
  launchURL.textContent = current.sessionUrl;
  launchPanel.hidden = false;
}

async function cancelLaunch(): Promise<void> {
  launchCancel.disabled = true;
  pairError.hidden = true;
  try {
    const response = await fetch(apiURL('/api/sunshine/cancel'), { method: 'POST' });
    const data = (await response.json()) as { ok?: boolean; error?: string };
    if (!response.ok || !data.ok) {
      throw new Error(data.error ?? 'Unable to stop Sunshine session');
    }
    launchPanel.hidden = true;
    await refreshSunshine();
  } catch (error) {
    pairError.textContent = String(error);
    pairError.hidden = false;
  } finally {
    launchCancel.disabled = false;
  }
}

sunshineRefresh.addEventListener('click', () => {
  void refreshSunshine();
});

sunshinePair.addEventListener('click', () => {
  void startPairing();
});

launchCancel.addEventListener('click', () => {
  void cancelLaunch();
});

connectButton.addEventListener('click', async () => {
  if (session) {
    inputController?.stop();
    inputController = null;

    await session.close();
    session = null;

    connectButton.textContent = 'Connect';
    fullscreenButton.disabled = true;
    mouseLockButton.disabled = true;
    emptyState.hidden = false;
    return;
  }

  const nextSession = new WebRtcSession(signalingInput.value.trim(), video);

  nextSession.onStateChange = (state) => {
    status.textContent = state[0].toUpperCase() + state.slice(1);
    status.dataset.state = state;

    const connected = state === 'connected';
    emptyState.hidden = connected;
    fullscreenButton.disabled = !connected;
    mouseLockButton.disabled = !connected;

    if (state === 'connected') {
      connectButton.textContent = 'Disconnect';
    } else if (state === 'connecting') {
      connectButton.textContent = 'Connecting…';
    } else {
      connectButton.textContent = 'Connect';
    }
  };

  nextSession.onStats = (stats) => {
    rtt.textContent = stats.rttMs === null ? '—' : `${stats.rttMs.toFixed(0)} ms`;
    bitrate.textContent = stats.bitrateMbps === null ? '—' : `${stats.bitrateMbps.toFixed(1)} Mbps`;
    fps.textContent = stats.fps === null ? '—' : stats.fps.toFixed(0);
    loss.textContent = stats.packetsLost === null ? '—' : String(stats.packetsLost);
  };

  nextSession.onError = (error) => {
    console.error('[StorPlay]', error);
    status.textContent = 'Error';
  };

  session = nextSession;
  inputController = new InputController(video, (message) => {
    nextSession.sendInput(message);
  });
  inputController.start();

  try {
    await nextSession.connect();
  } catch (error) {
    console.error(error);
    inputController.stop();
    inputController = null;
    await nextSession.close();
    session = null;
    connectButton.textContent = 'Connect';
    emptyState.hidden = false;
  }
});

fullscreenButton.addEventListener('click', () => {
  void video.requestFullscreen();
});

mouseLockButton.addEventListener('click', () => {
  void inputController?.requestPointerLock();
});

void refreshSunshine();
