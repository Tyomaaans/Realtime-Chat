import type { ServerEvent } from "../types/chat";

function object(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function keys(value: Record<string, unknown>, allowed: string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function text(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function date(value: unknown): boolean {
  return text(value) && value.endsWith("Z") && Number.isFinite(Date.parse(value));
}

function oneOf(value: unknown, allowed: string[]): boolean {
  return typeof value === "string" && allowed.includes(value);
}

export function parseServerEvent(data: unknown): ServerEvent | null {
  if (typeof data !== "string") return null;
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }
  return isServerEvent(value) ? value : null;
}

function isServerEvent(value: unknown): value is ServerEvent {
  if (!object(value) || !keys(value, ["type", "request_id", "room_id", "payload"]) || !text(value.type) || !object(value.payload)) return false;
  if (value.request_id !== undefined && (!text(value.request_id) || Array.from(value.request_id).length > 64)) return false;
  const payload = value.payload;
  const room = text(value.room_id);
  let valid = false;
  switch (value.type) {
    case "session.ready":
      valid = keys(payload, ["state"]) && payload.state === "idle" && value.room_id === undefined;
      break;
    case "queue.waiting":
      valid = keys(payload, ["state", "expires_at"]) && payload.state === "waiting" && date(payload.expires_at) && value.room_id === undefined;
      break;
    case "queue.cancelled":
      valid = keys(payload, ["state", "reason"]) && payload.state === "idle" && oneOf(payload.reason, ["cancelled", "queue_timeout", "shutdown"]) && value.room_id === undefined;
      break;
    case "chat.matched":
      valid = room && keys(payload, ["state"]) && payload.state === "matched";
      break;
    case "chat.message":
      valid = room && keys(payload, ["message_id", "text", "sender", "sent_at"]) && text(payload.message_id) && text(payload.text) && oneOf(payload.sender, ["self", "peer"]) && date(payload.sent_at);
      break;
    case "chat.typing":
      valid = room && keys(payload, ["typing", "sender"]) && typeof payload.typing === "boolean" && oneOf(payload.sender, ["self", "peer"]);
      break;
    case "peer.left":
      valid = room && keys(payload, ["state", "reason"]) && payload.state === "idle" && oneOf(payload.reason, ["stopped", "next", "disconnected", "slow_consumer"]);
      break;
    case "error":
      valid = keys(payload, ["code", "message", "state", "retry_after_ms"])
        && oneOf(payload.code, ["invalid_payload", "unknown_event", "invalid_state", "stale_room", "rate_limited", "queue_full", "server_busy", "internal_error"])
        && text(payload.message)
        && (payload.state === undefined || oneOf(payload.state, ["idle", "waiting", "matched"]))
        && (value.room_id === undefined || room)
        && (payload.retry_after_ms === undefined || (typeof payload.retry_after_ms === "number" && Number.isFinite(payload.retry_after_ms) && payload.retry_after_ms >= 0));
      break;
  }
  return valid;
}
