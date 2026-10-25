import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MessageList from "./message-list";
import { initialState } from "../state";
import type { ChatSessionState } from "../types";

describe("transcript", () => {
  const state: ChatSessionState = {
    ...initialState,
    connection: "open",
    chat: "matched",
    roomId: "room",
    messages: [
      {
        message_id: "m1",
        sender: "peer",
        text: '<img src=x onerror="window.executed=true">',
        sent_at: "2026-10-24T10:00:00Z",
      },
    ],
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
    rerender(
      <MessageList
        state={{
          ...state,
          messages: [...state.messages, { ...state.messages[0], message_id: "m2" }],
        }}
        {...actions}
      />,
    );
    expect(log.scrollTop).toBe(100);
    fireEvent.click(screen.getByRole("button", { name: "New messages" }));
    expect(log.scrollTop).toBe(1000);
  });
});
