export type InputMessage =
  | { type: 'key'; code: string; key: string; down: boolean; repeat: boolean }
  | { type: 'mouse_move'; dx: number; dy: number }
  | { type: 'mouse_button'; button: number; down: boolean }
  | { type: 'mouse_wheel'; dx: number; dy: number }
  | { type: 'gamepad'; index: number; buttons: number[]; axes: number[]; timestamp: number }
  | { type: 'release_all' };

export type ClientSignal =
  | { type: 'answer'; sdp: RTCSessionDescriptionInit }
  | { type: 'ice'; candidate: RTCIceCandidateInit };

export type HostSignal =
  | { type: 'offer'; sdp: RTCSessionDescriptionInit }
  | { type: 'ice'; candidate: RTCIceCandidateInit }
  | { type: 'error'; message: string }
  | { type: 'session-ended'; reason?: string };
