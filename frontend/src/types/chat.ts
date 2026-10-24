export type ConnectionState = "connecting" | "open" | "closed" | "reconnecting";
export type ChatState = "idle" | "waiting" | "matched";
export type CommandType = "queue.join" | "queue.cancel" | "chat.send" | "chat.typing" | "chat.stop" | "chat.next";

export interface Message {
  message_id: string;
  text: string;
  sender: "self" | "peer";
  sent_at: string;
}

interface Envelope {
  request_id?: string;
}

export type ServerEvent = Envelope & (
  | { type: "session.ready"; payload: { state: "idle" } }
  | { type: "queue.waiting"; payload: { state: "waiting"; expires_at: string } }
  | { type: "queue.cancelled"; payload: { state: "idle"; reason: "cancelled" | "queue_timeout" | "shutdown" } }
  | { type: "chat.matched"; room_id: string; payload: { state: "matched" } }
  | { type: "chat.message"; room_id: string; payload: Message }
  | { type: "chat.typing"; room_id: string; payload: { typing: boolean; sender: "self" | "peer" } }
  | { type: "peer.left"; room_id: string; payload: { state: "idle"; reason: "stopped" | "next" | "disconnected" | "slow_consumer" } }
  | { type: "error"; room_id?: string; payload: { code: string; message: string; state?: ChatState; retry_after_ms?: number } }
);

export interface ChatStateData {
  connection: ConnectionState;
  chat: ChatState;
  roomId: string | null;
  messages: Message[];
  removed: boolean;
  peerTyping: boolean;
  pending: { type: CommandType; requestId: string } | null;
  notice: string;
  error: string;
  retryUntil: number;
  attempt: number;
  contentVersion: number;
}
