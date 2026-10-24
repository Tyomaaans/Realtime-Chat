import { TRANSCRIPT_LIMIT } from "../config/chat";
import type { ChatStateData, CommandType, ConnectionState, ServerEvent } from "../types/chat";

export const initialState: ChatStateData = {
  connection: "connecting",
  chat: "idle",
  roomId: null,
  messages: [],
  removed: false,
  peerTyping: false,
  pending: null,
  notice: "",
  error: "",
  retryUntil: 0,
  attempt: 0,
  contentVersion: 0,
};

export type ChatAction =
  | { type: "connection"; connection: ConnectionState; attempt?: number }
  | { type: "pending"; command: CommandType; requestId: string }
  | { type: "event"; event: ServerEvent; now: number }
  | { type: "typing_expired" }
  | { type: "rate_limit_expired" }
  | { type: "error"; message: string };

function clearContent(state: ChatStateData): ChatStateData {
  return { ...state, messages: [], removed: false, peerTyping: false, contentVersion: state.contentVersion + 1 };
}

export function chatReducer(state: ChatStateData, action: ChatAction): ChatStateData {
  switch (action.type) {
    case "connection":
      return {
        ...clearContent(state),
        connection: action.connection,
        attempt: action.attempt ?? 0,
        chat: "idle",
        roomId: null,
        pending: null,
        notice: "",
        error: "",
        retryUntil: 0,
      };
    case "pending": {
      const next = ["chat.next", "chat.stop"].includes(action.command) ? clearContent(state) : state;
      return { ...next, pending: { type: action.command, requestId: action.requestId }, error: "", notice: "" };
    }
    case "error":
      return { ...state, error: action.message };
    case "typing_expired":
      return { ...state, peerTyping: false };
    case "rate_limit_expired":
      return { ...state, retryUntil: 0 };
    case "event": {
      const event = action.event;
      if ("room_id" in event && event.type !== "chat.matched" && event.room_id !== undefined && event.room_id !== state.roomId) return state;
      const pending = event.request_id === state.pending?.requestId ? null : state.pending;
      switch (event.type) {
        case "session.ready":
          return {
            ...clearContent(state),
            connection: "open",
            chat: "idle",
            roomId: null,
            pending: null,
            error: "",
            attempt: 0,
          };
        case "queue.waiting":
          if (state.chat === "matched" && state.pending?.type !== "chat.next") return state;
          return { ...clearContent(state), chat: "waiting", roomId: null, pending, notice: "", error: "" };
        case "queue.cancelled":
          return {
            ...clearContent(state),
            chat: "idle",
            roomId: null,
            pending: null,
            notice: event.payload.reason === "queue_timeout" ? "The search timed out. You can try again." : "Search ended.",
          };
        case "chat.matched":
          if (state.chat !== "waiting" && state.pending?.type !== "queue.join" && state.pending?.type !== "chat.next") return state;
          return { ...clearContent(state), chat: "matched", roomId: event.room_id, pending: null, notice: "", error: "" };
        case "chat.message": {
          if (state.chat !== "matched" || state.pending?.type === "chat.next" || state.pending?.type === "chat.stop") return state;
          const messages = [...state.messages, event.payload];
          return {
            ...state,
            messages: messages.slice(-TRANSCRIPT_LIMIT),
            removed: state.removed || messages.length > TRANSCRIPT_LIMIT,
            peerTyping: event.payload.sender === "peer" ? false : state.peerTyping,
          };
        }
        case "chat.typing":
          return event.payload.sender === "peer" && !state.pending ? { ...state, peerTyping: event.payload.typing } : state;
        case "peer.left":
          return {
            ...clearContent(state),
            chat: "idle",
            roomId: null,
            pending: null,
            notice: event.request_id ? "You ended the conversation. Your messages have been cleared." : "The other person left. Your messages have been cleared.",
          };
        case "error": {
          const next: ChatStateData = event.payload.state === "idle" && state.chat !== "idle"
            ? { ...clearContent(state), chat: "idle", roomId: null, pending: null }
            : { ...state, pending };
          return { ...next, error: event.payload.message, retryUntil: event.payload.retry_after_ms ? action.now + event.payload.retry_after_ms : 0 };
        }
      }
    }
  }
}
