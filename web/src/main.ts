import { InputController } from './input';
import { WebRtcSession } from './webrtc';

const video = document.querySelector<HTMLVideoElement>('#player')!;
const status = document.querySelector<HTMLElement>('#status')!;
const connectButton = document.querySelector<HTMLButtonElement>('#connect')!;
const fullscreenButton = document.querySelector<HTMLButtonElement>('#fullscreen')!;
const mouseLockButton = document.querySelector<HTMLButtonElement>('#mouse-lock')!;
const signalingInput = document.querySelector<HTMLInputElement>('#signaling-url')!;
const emptyState = document.querySelector<HTMLElement>('#empty-state')!;

const rtt = document.querySelector<HTMLElement>('#rtt')!;
const bitrate = document.querySelector<HTMLElement>('#bitrate')!;
const fps = document.querySelector<HTMLElement>('#fps')!;
const loss = document.querySelector<HTMLElement>('#loss')!;

let session: WebRtcSession | null = null;
let inputController: InputController | null = null;

connectButton.addEventListener('click', async () => {
  if (session) {
    inputController?.stop();
    inputController = null;

    await session.close();
    session = null;

    connectButton.textContent = 'Connect';
    fullscreenButton.disabled = true;
    mouseLockButton.disabled = true;
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
    connectButton.textContent = connected ? 'Disconnect' : 'Connecting…';
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
  }
});

fullscreenButton.addEventListener('click', () => {
  void video.requestFullscreen();
});

mouseLockButton.addEventListener('click', () => {
  void inputController?.requestPointerLock();
});
