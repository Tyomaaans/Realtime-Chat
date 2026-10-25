import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import MessageComposer from "./message-composer";

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
