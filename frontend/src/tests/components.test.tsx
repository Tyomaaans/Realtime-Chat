import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import MessageComposer from "../components/MessageComposer";
import MessageList from "../components/MessageList";
import { initialState } from "../hooks/chat-state";
import type { ChatStateData } from "../types/chat";

describe("composer", () => {
  it("supports Enter, Shift+Enter, IME composition, and focus after sending", async () => {
    const send = vi.fn(() => true);
    const user = userEvent.setup();
    render(<MessageComposer enabled waiting={false} onSend={send} onTyping={vi.fn()} />);
    const input = screen.getByRole("textbox", { name: "Your message" });
    await user.type(input, "Hello");
    await user.keyboard("{Shift>}{Enter}{/Shift}");
    expect(input).toHaveValue("Hello\n");
    expect(send).not.toHaveBeenCalled();
    fireEvent.compositionStart(input);
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    expect(send).not.toHaveBeenCalled();
    fireEvent.compositionEnd(input);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(send).toHaveBeenCalledExactlyOnceWith("Hello");
    expect(input).toHaveValue("");
    expect(input).toHaveFocus();
  });

  it("counts Unicode code points and rejects text above the limit", () => {
    const send = vi.fn(() => true);
    render(<MessageComposer enabled waiting={false} onSend={send} onTyping={vi.fn()} />);
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "\u754c".repeat(2001) } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(send).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("at most 2,000");
  });
});

describe("transcript", () => {
  const state: ChatStateData = {
    ...initialState, connection: "open", chat: "matched", roomId: "room",
    messages: [{ message_id: "m1", sender: "peer", text: '<img src=x onerror="window.executed=true">', sent_at: "2026-10-24T10:00:00Z" }],
  };
  const actions = { onStart: vi.fn(), onCancel: vi.fn(), onRetry: vi.fn() };

  it("renders submitted HTML as plain text", () => {
    render(<MessageList state={state} {...actions} />);
    expect(screen.getByText(state.messages[0].text)).toBeVisible();
    expect(document.querySelector("img")).toBeNull();
  });

  it("preserves scroll position while older messages are being read", () => {
    const { rerender } = render(<MessageList state={state} {...actions} />);
    const log = screen.getByRole("log");
    Object.defineProperty(log, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(log, "clientHeight", { configurable: true, value: 200 });
    log.scrollTop = 100;
    fireEvent.scroll(log);
    rerender(<MessageList state={{ ...state, messages: [...state.messages, { ...state.messages[0], message_id: "m2" }] }} {...actions} />);
    expect(log.scrollTop).toBe(100);
    fireEvent.click(screen.getByRole("button", { name: "New messages" }));
    expect(log.scrollTop).toBe(1000);
  });
});
