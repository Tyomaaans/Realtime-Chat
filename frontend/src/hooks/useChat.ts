import { useEffect, useReducer, useRef, useState } from "react";
import { FRAME_BYTES, RECONNECT_LIMIT, TYPING_TIMEOUT, socketAddress } from "../config/chat";
import type { CommandType } from "../types/chat";
import { parseServerEvent } from "../utils/chat-protocol";
import { chatReducer, initialState } from "./chat-state";

export default function useChat() {
  const [state, dispatch] = useReducer(chatReducer, initialState);
  const [revision, setRevision] = useState(0);
  const socketRef = useRef<WebSocket | null>(null);
  const requestRef = useRef(0);
  const stateRef = useRef(state);
  const lastTypingRef = useRef(0);
  stateRef.current = state;

  useEffect(() => {
    let active = true;
    let generation = 0;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let typingTimer: ReturnType<typeof setTimeout> | undefined;
    let readyTimer: ReturnType<typeof setTimeout> | undefined;
    let socket: WebSocket | null = null;

    const connect = (attempt: number) => {
      if (!active) return;
      const current = ++generation;
      dispatch({ type: "connection", connection: attempt ? "reconnecting" : "connecting", attempt });
      socket = new WebSocket(socketAddress());
      socketRef.current = socket;
      const connection = socket;
      let ready = false;
      const valid = () => active && generation === current;
      readyTimer = setTimeout(() => { if (valid()) connection.close(); }, 10000);
      connection.onmessage = (message) => {
        if (!valid()) return;
        const event = parseServerEvent(message.data);
        if (!event) {
          connection.close(1000, "Invalid server event");
          return;
        }
        if (event.type === "session.ready") {
          ready = true;
          clearTimeout(readyTimer);
        }
        dispatch({ type: "event", event, now: Date.now() });
        if (event.type === "chat.typing" && event.payload.sender === "peer" && event.room_id === stateRef.current.roomId) {
          clearTimeout(typingTimer);
          typingTimer = setTimeout(() => { if (valid()) dispatch({ type: "typing_expired" }); }, TYPING_TIMEOUT);
        }
      };
      connection.onclose = () => {
        if (!valid()) return;
        socketRef.current = null;
        clearTimeout(typingTimer);
        clearTimeout(readyTimer);
        const nextAttempt = ready ? 1 : attempt + 1;
        if (nextAttempt > RECONNECT_LIMIT) {
          dispatch({ type: "connection", connection: "closed", attempt: RECONNECT_LIMIT });
          return;
        }
        dispatch({ type: "connection", connection: "reconnecting", attempt: nextAttempt });
        const delay = Math.min(15000, 1000 * 2 ** (nextAttempt - 1)) * (0.8 + Math.random() * 0.2);
        reconnectTimer = setTimeout(() => connect(nextAttempt), delay);
      };
      connection.onerror = () => { if (valid()) connection.close(); };
    };

    const close = () => {
      active = false;
      generation++;
      clearTimeout(reconnectTimer);
      clearTimeout(typingTimer);
      clearTimeout(readyTimer);
      clearTimeout(startTimer);
      socketRef.current = null;
      if (socket) {
        socket.onmessage = null;
        socket.onclose = null;
        socket.onerror = null;
        socket.close(1000);
      }
    };
    const startTimer = setTimeout(() => connect(0), 0);
    const hide = () => { dispatch({ type: "connection", connection: "closed" }); close(); };
    const restore = (event: PageTransitionEvent) => { if (event.persisted) setRevision((value) => value + 1); };
    window.addEventListener("pagehide", hide);
    window.addEventListener("pageshow", restore);
    return () => {
      close();
      window.removeEventListener("pagehide", hide);
      window.removeEventListener("pageshow", restore);
    };
  }, [revision]);

  useEffect(() => {
    if (!state.pending) return;
    const timeout = setTimeout(() => socketRef.current?.close(), 8000);
    return () => clearTimeout(timeout);
  }, [state.pending]);

  useEffect(() => {
    if (!state.retryUntil) return;
    const timeout = setTimeout(() => dispatch({ type: "rate_limit_expired" }), Math.max(0, state.retryUntil - Date.now()));
    return () => clearTimeout(timeout);
  }, [state.retryUntil]);

  const command = (type: CommandType, payload: Record<string, unknown> = {}): boolean => {
    const current = stateRef.current;
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || current.connection !== "open" || current.pending || Date.now() < current.retryUntil) return false;
    if (type.startsWith("chat.") && current.chat !== "matched") return false;
    if (type === "queue.join" && current.chat !== "idle") return false;
    if (type === "queue.cancel" && current.chat !== "waiting") return false;
    const requestId = `c${++requestRef.current}`;
    const envelope = { type, request_id: requestId, ...(type.startsWith("chat.") ? { room_id: current.roomId } : {}), payload };
    const data = JSON.stringify(envelope);
    if (new TextEncoder().encode(data).length > FRAME_BYTES) {
      dispatch({ type: "error", message: "This message is too large after encoding. Please shorten it." });
      return false;
    }
    try {
      socket.send(data);
    } catch {
      dispatch({ type: "error", message: "The connection closed before your message could be sent." });
      socket.close();
      return false;
    }
    if (!["chat.send", "chat.typing"].includes(type)) {
      dispatch({ type: "pending", command: type, requestId });
      stateRef.current = { ...current, pending: { type, requestId } };
    }
    return true;
  };

  const typing = (value: boolean) => {
    if (Date.now() - lastTypingRef.current < 600) return;
    if (command("chat.typing", { typing: value })) lastTypingRef.current = Date.now();
  };

  return {
    state,
    join: () => command("queue.join"),
    cancel: () => command("queue.cancel"),
    send: (text: string) => command("chat.send", { text }),
    stop: () => command("chat.stop"),
    next: () => command("chat.next"),
    typing,
    retry: () => setRevision((value) => value + 1),
  };
}
