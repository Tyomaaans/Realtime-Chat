import { StrictMode } from "react";
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import useChat from "../hooks/useChat";

class FakeSocket {
  static OPEN = 1;
  static instances: FakeSocket[] = [];
  readyState = 0;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  sent: string[] = [];

  constructor() { FakeSocket.instances.push(this); }
  send(data: string) { this.sent.push(data); }
  close = vi.fn(() => { this.readyState = 3; this.onclose?.(); });
  ready() { this.readyState = 1; this.event({ type: "session.ready", payload: { state: "idle" } }); }
  event(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) }); }
}

beforeEach(() => {
  vi.useFakeTimers();
  FakeSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeSocket);
  vi.spyOn(Math, "random").mockReturnValue(1);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function socket() { return FakeSocket.instances[FakeSocket.instances.length - 1]; }

function mountChat(strict = false) {
  const hook = renderHook(() => useChat(), strict ? { wrapper: StrictMode } : undefined);
  act(() => vi.advanceTimersByTime(0));
  return hook;
}

describe("socket lifecycle", () => {
  it("cleans StrictMode sockets and ignores stale callbacks", () => {
    const { result, unmount } = mountChat(true);
    expect(FakeSocket.instances).toHaveLength(1);
    const current = socket();
    act(() => current.ready());
    const stale = current.onmessage;
    act(() => result.current.retry());
    act(() => vi.advanceTimersByTime(0));
    act(() => stale?.({ data: JSON.stringify({ type: "session.ready", payload: { state: "idle" } }) }));
    expect(result.current.state.connection).toBe("connecting");
    unmount();
    expect(socket().close).toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("stops after five reconnects and offers a fresh manual retry", () => {
    const { result } = mountChat();
    for (const delay of [1000, 2000, 4000, 8000, 15000]) {
      act(() => socket().close());
      act(() => vi.advanceTimersByTime(delay));
    }
    act(() => socket().close());
    expect(FakeSocket.instances).toHaveLength(6);
    expect(result.current.state.connection).toBe("closed");
    act(() => vi.advanceTimersByTime(60000));
    expect(FakeSocket.instances).toHaveLength(6);
    act(() => result.current.retry());
    act(() => vi.advanceTimersByTime(0));
    expect(FakeSocket.instances).toHaveLength(7);
  });

  it("sends once, waits for echo, clears content on disconnect, and never resends", () => {
    const { result } = mountChat();
    const current = socket();
    act(() => current.ready());
    act(() => { result.current.join(); });
    act(() => current.event({ type: "chat.matched", request_id: "c1", room_id: "room", payload: { state: "matched" } }));
    act(() => { expect(result.current.send("Hello")).toBe(true); });
    expect(result.current.state.messages).toHaveLength(0);
    act(() => current.event({ type: "chat.message", request_id: "c2", room_id: "room", payload: { message_id: "m1", text: "Hello", sender: "self", sent_at: "2026-10-24T10:00:00Z" } }));
    expect(result.current.state.messages).toHaveLength(1);
    act(() => current.close());
    expect(result.current.state.messages).toHaveLength(0);
    act(() => { expect(result.current.send("offline")).toBe(false); });
    act(() => vi.advanceTimersByTime(1000));
    act(() => socket().ready());
    expect(result.current.state.chat).toBe("idle");
    expect(socket().sent).toHaveLength(0);
  });

  it("expires peer typing and rate-limit delays", () => {
    const { result } = mountChat();
    act(() => socket().ready());
    act(() => { result.current.join(); });
    act(() => socket().event({ type: "chat.matched", request_id: "c1", room_id: "room", payload: { state: "matched" } }));
    act(() => socket().event({ type: "chat.typing", room_id: "room", payload: { sender: "peer", typing: true } }));
    expect(result.current.state.peerTyping).toBe(true);
    act(() => vi.advanceTimersByTime(3000));
    expect(result.current.state.peerTyping).toBe(false);
    act(() => socket().event({ type: "error", room_id: "room", payload: { code: "rate_limited", message: "Please wait", retry_after_ms: 500 } }));
    act(() => { expect(result.current.send("wait")).toBe(false); });
    act(() => vi.advanceTimersByTime(500));
    expect(result.current.state.retryUntil).toBe(0);
  });

  it("closes malformed server events with a browser-permitted close code", () => {
    mountChat();
    const current = socket();
    act(() => current.ready());
    act(() => current.event({ type: "unknown", payload: {} }));
    expect(current.close).toHaveBeenCalledWith(1000, "Invalid server event");
  });
});
