import { describe, expect, it } from "vitest";
import type { Message } from "./types";
import { parseServerEvent } from "./protocol";

const message: Message = {
  message_id: "m1",
  sender: "peer",
  text: "Hello",
  sent_at: "2026-10-24T10:00:00Z",
};

describe("server event parsing", () => {
  it.each([
    "null",
    "[]",
    "{}",
    "{",
    JSON.stringify({ type: "chat.message", room_id: "room", payload: { ...message, text: 7 } }),
    JSON.stringify({
      type: "chat.typing",
      room_id: "room",
      payload: { typing: "true", sender: "peer" },
    }),
    JSON.stringify({ type: "session.ready", payload: { state: "matched" } }),
    JSON.stringify({ type: "session.ready", payload: { state: "idle" }, extra: true }),
    JSON.stringify({
      type: "error",
      payload: { code: "rate_limited", message: "Wait", retry_after_ms: -1 },
    }),
    JSON.stringify({
      type: "chat.typing",
      room_id: "room",
      payload: { typing: true, sender: { toString: null } },
    }),
  ])("rejects invalid input %s", (data) => {
    expect(parseServerEvent(data)).toBeNull();
  });

  it("validates messages and explicit false typing values", () => {
    expect(
      parseServerEvent(JSON.stringify({ type: "chat.message", room_id: "room", payload: message }))
        ?.type,
    ).toBe("chat.message");
    expect(
      parseServerEvent(
        JSON.stringify({
          type: "chat.typing",
          room_id: "room",
          payload: { typing: false, sender: "peer" },
        }),
      )?.type,
    ).toBe("chat.typing");
    expect(parseServerEvent(new Blob())).toBeNull();
  });
});
