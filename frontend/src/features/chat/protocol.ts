import type { ServerEvent } from "./types";

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(value: Record<string, unknown>, allowed: string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function isTimestamp(value: unknown): boolean {
  return isNonEmptyString(value) && value.endsWith("Z") && Number.isFinite(Date.parse(value));
}

function isOneOf(value: unknown, allowed: string[]): boolean {
  return typeof value === "string" && allowed.includes(value);
}

export function parseServerEvent(data: unknown): ServerEvent | null {
  if (typeof data !== "string") {
    return null;
  }

  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }

  return isServerEvent(value) ? value : null;
}

function isServerEvent(value: unknown): value is ServerEvent {
  if (
    !isObject(value) ||
    !hasOnlyKeys(value, ["type", "request_id", "room_id", "payload"]) ||
    !isNonEmptyString(value.type) ||
    !isObject(value.payload)
  ) {
    return false;
  }

  if (
    value.request_id !== undefined &&
    (!isNonEmptyString(value.request_id) || Array.from(value.request_id).length > 64)
  ) {
    return false;
  }

  const payload = value.payload;
  const room = isNonEmptyString(value.room_id);
  switch (value.type) {
    case "session.ready":
      return (
        hasOnlyKeys(payload, ["state"]) && payload.state === "idle" && value.room_id === undefined
      );
    case "queue.waiting":
      return (
        hasOnlyKeys(payload, ["state", "expires_at"]) &&
        payload.state === "waiting" &&
        isTimestamp(payload.expires_at) &&
        value.room_id === undefined
      );
    case "queue.cancelled":
      return (
        hasOnlyKeys(payload, ["state", "reason"]) &&
        payload.state === "idle" &&
        isOneOf(payload.reason, ["cancelled", "queue_timeout", "shutdown"]) &&
        value.room_id === undefined
      );
    case "chat.matched":
      return room && hasOnlyKeys(payload, ["state"]) && payload.state === "matched";
    case "chat.message":
      return (
        room &&
        hasOnlyKeys(payload, ["message_id", "text", "sender", "sent_at"]) &&
        isNonEmptyString(payload.message_id) &&
        isNonEmptyString(payload.text) &&
        isOneOf(payload.sender, ["self", "peer"]) &&
        isTimestamp(payload.sent_at)
      );
    case "chat.typing":
      return (
        room &&
        hasOnlyKeys(payload, ["typing", "sender"]) &&
        typeof payload.typing === "boolean" &&
        isOneOf(payload.sender, ["self", "peer"])
      );
    case "peer.left":
      return (
        room &&
        hasOnlyKeys(payload, ["state", "reason"]) &&
        payload.state === "idle" &&
        isOneOf(payload.reason, ["stopped", "next", "disconnected", "slow_consumer"])
      );
    case "error":
      return (
        hasOnlyKeys(payload, ["code", "message", "state", "retry_after_ms"]) &&
        isOneOf(payload.code, [
          "invalid_payload",
          "unknown_event",
          "invalid_state",
          "stale_room",
          "rate_limited",
          "queue_full",
          "server_busy",
          "internal_error",
        ]) &&
        isNonEmptyString(payload.message) &&
        (payload.state === undefined || isOneOf(payload.state, ["idle", "waiting", "matched"])) &&
        (value.room_id === undefined || room) &&
        (payload.retry_after_ms === undefined ||
          (typeof payload.retry_after_ms === "number" &&
            Number.isFinite(payload.retry_after_ms) &&
            payload.retry_after_ms >= 0))
      );
    default:
      return false;
  }
}
