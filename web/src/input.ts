import type { InputMessage } from './protocol';

type SendInput = (message: InputMessage) => void;

export class InputController {
  private gamepadFrame: number | null = null;
  private lastGamepadPayload = new Map<number, string>();

  constructor(
    private readonly target: HTMLElement,
    private readonly send: SendInput,
  ) {}

  start(): void {
    window.addEventListener('keydown', this.onKeyDown, { capture: true });
    window.addEventListener('keyup', this.onKeyUp, { capture: true });
    window.addEventListener('blur', this.releaseAll);

    this.target.addEventListener('mousemove', this.onMouseMove);
    this.target.addEventListener('mousedown', this.onMouseDown);
    this.target.addEventListener('mouseup', this.onMouseUp);
    this.target.addEventListener('wheel', this.onWheel, { passive: false });
    this.target.addEventListener('contextmenu', this.preventContextMenu);

    this.pollGamepads();
  }

  stop(): void {
    window.removeEventListener('keydown', this.onKeyDown, { capture: true });
    window.removeEventListener('keyup', this.onKeyUp, { capture: true });
    window.removeEventListener('blur', this.releaseAll);

    this.target.removeEventListener('mousemove', this.onMouseMove);
    this.target.removeEventListener('mousedown', this.onMouseDown);
    this.target.removeEventListener('mouseup', this.onMouseUp);
    this.target.removeEventListener('wheel', this.onWheel);
    this.target.removeEventListener('contextmenu', this.preventContextMenu);

    if (this.gamepadFrame !== null) {
      cancelAnimationFrame(this.gamepadFrame);
      this.gamepadFrame = null;
    }

    this.releaseAll();
  }

  async requestPointerLock(): Promise<void> {
    await this.target.requestPointerLock();
  }

  private onKeyDown = (event: KeyboardEvent): void => {
    if (event.repeat) event.preventDefault();

    this.send({
      type: 'key',
      code: event.code,
      key: event.key,
      down: true,
      repeat: event.repeat,
    });
  };

  private onKeyUp = (event: KeyboardEvent): void => {
    this.send({
      type: 'key',
      code: event.code,
      key: event.key,
      down: false,
      repeat: false,
    });
  };

  private onMouseMove = (event: MouseEvent): void => {
    if (document.pointerLockElement !== this.target) return;
    if (event.movementX === 0 && event.movementY === 0) return;

    this.send({
      type: 'mouse_move',
      dx: event.movementX,
      dy: event.movementY,
    });
  };

  private onMouseDown = (event: MouseEvent): void => {
    if (document.pointerLockElement !== this.target) return;
    event.preventDefault();
    this.send({ type: 'mouse_button', button: event.button, down: true });
  };

  private onMouseUp = (event: MouseEvent): void => {
    if (document.pointerLockElement !== this.target) return;
    event.preventDefault();
    this.send({ type: 'mouse_button', button: event.button, down: false });
  };

  private onWheel = (event: WheelEvent): void => {
    if (document.pointerLockElement !== this.target) return;
    event.preventDefault();
    this.send({ type: 'mouse_wheel', dx: event.deltaX, dy: event.deltaY });
  };

  private preventContextMenu = (event: Event): void => {
    if (document.pointerLockElement === this.target) event.preventDefault();
  };

  private releaseAll = (): void => {
    this.send({ type: 'release_all' });
  };

  private pollGamepads = (): void => {
    for (const gamepad of navigator.getGamepads()) {
      if (!gamepad) continue;

      const message: InputMessage = {
        type: 'gamepad',
        index: gamepad.index,
        buttons: gamepad.buttons.map((button) => button.value),
        axes: Array.from(gamepad.axes),
        timestamp: gamepad.timestamp,
      };

      const payload = JSON.stringify(message);
      if (this.lastGamepadPayload.get(gamepad.index) !== payload) {
        this.lastGamepadPayload.set(gamepad.index, payload);
        this.send(message);
      }
    }

    this.gamepadFrame = requestAnimationFrame(this.pollGamepads);
  };
}
