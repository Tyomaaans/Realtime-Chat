import { describe, expect, it } from "vitest";
import { chatReducer, initialState } from "./state";
import type { ChatAction } from "./state";
import type { ChatSessionState, Message, ServerEvent } from "./types";

const message: Message = {
  message_id: "m1",
  sender: "peer",
  text: "Hello",
  sent_at: "2026-10-24T10:00:00Z",
};
const matched: ChatSessionState = {
  ...initialState,
  connection: "open",
  chat: "matched",
  roomId: "room",
  messages: [message],
};

describe("chat state", () => {
  it("retains only the latest 200 messages and announces removal", () => {
    let state = matched;
    for (let i = 0; i < 201; i++) {
      state = chatReducer(state, {
        type: "event",
        now: 0,
        event: {
          type: "chat.message",
          room_id: "room",
          payload: { ...message, message_id: `m${i + 2}` },
        },
      });
    }

    expect(state.messages).toHaveLength(200);
    expect(state.messages[0].message_id).toBe("m3");
    expect(state.removed).toBe(true);
  });

  it.each<ChatAction>([
    { type: "connection", connection: "closed" },
    { type: "connection", connection: "reconnecting", attempt: 1 },
    { type: "pending", command: "chat.next", requestId: "next" },
    { type: "pending", command: "chat.stop", requestId: "stop" },
    {
      type: "event",
      now: 0,
      event: {
        type: "peer.left",
        room_id: "room",
        payload: { state: "idle", reason: "disconnected" },
      },
    },
  ])("clears private content on $type", (action) => {
    const result = chatReducer({ ...matched, peerTyping: true }, action);
    expect(result.messages).toEqual([]);
    expect(result.peerTyping).toBe(false);
    expect(result.contentVersion).toBeGreaterThan(matched.contentVersion);
  });

  it("ignores old rooms and rejects unsolicited matches", () => {
    const event: ServerEvent = {
      type: "peer.left",
      room_id: "old-room",
      payload: { state: "idle", reason: "next" },
    };
    expect(chatReducer(matched, { type: "event", event, now: 0 })).toBe(matched);
    expect(
      chatReducer(initialState, {
        type: "event",
        now: 0,
        event: { type: "chat.matched", room_id: "unexpected", payload: { state: "matched" } },
      }),
    ).toBe(initialState);
  });

  it("waits for the server echo before adding outgoing messages", () => {
    const state = chatReducer(matched, {
      type: "pending",
      command: "chat.next",
      requestId: "next",
    });
    const next = chatReducer(state, {
      type: "event",
      now: 0,
      event: {
        type: "chat.matched",
        room_id: "new-room",
        request_id: "next",
        payload: { state: "matched" },
      },
    });
    expect(next.roomId).toBe("new-room");
    expect(next.messages).toEqual([]);
    expect(next.pending).toBeNull();
  });

  it("returns to idle when Next cannot enter a full queue", () => {
    const pending = chatReducer(matched, {
      type: "pending",
      command: "chat.next",
      requestId: "next",
    });
    const result = chatReducer(pending, {
      type: "event",
      now: 0,
      event: {
        type: "error",
        request_id: "next",
        payload: { code: "queue_full", message: "Queue full", state: "idle" },
      },
    });
    expect(result.chat).toBe("idle");
    expect(result.roomId).toBeNull();
    expect(result.pending).toBeNull();
  });
});
