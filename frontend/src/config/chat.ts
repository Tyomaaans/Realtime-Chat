export const CHAT_PATH = "/v1/chat/ws";
export const MESSAGE_LIMIT = 2000;
export const MESSAGE_BYTES = 8000;
export const FRAME_BYTES = 8192;
export const TRANSCRIPT_LIMIT = 200;
export const TYPING_TIMEOUT = 3000;
export const RECONNECT_LIMIT = 5;

export function socketAddress(): string {
  const address = new URL(CHAT_PATH, window.location.origin);
  address.protocol = address.protocol === "https:" ? "wss:" : "ws:";
  return address.toString();
}
