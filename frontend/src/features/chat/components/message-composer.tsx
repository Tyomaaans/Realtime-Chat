import { useEffect, useId, useRef, useState } from "react";
import { MESSAGE_BYTES, MESSAGE_LIMIT } from "../config";
import Button from "../../../components/ui/button";

interface MessageComposerProps {
  enabled: boolean;
  waiting: boolean;
  onSend: (text: string) => boolean;
  onTyping: (typing: boolean) => void;
}

export default function MessageComposer({
  enabled,
  waiting,
  onSend,
  onTyping,
}: MessageComposerProps) {
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const composingRef = useRef(false);
  const id = useId();
  const characters = Array.from(draft).length;

  useEffect(() => {
    if (enabled) {
      inputRef.current?.focus();
    }
  }, [enabled]);

  const submit = () => {
    if (!enabled || composingRef.current) {
      return;
    }

    const text = draft.trim();
    if (!text) {
      setError("Write a message before sending.");
      return;
    }

    if (
      Array.from(text).length > MESSAGE_LIMIT ||
      new TextEncoder().encode(text).length > MESSAGE_BYTES
    ) {
      setError("Use at most 2,000 characters and 8,000 bytes.");
      return;
    }

    if (onSend(text)) {
      setDraft("");
      setError("");
      onTyping(false);
      inputRef.current?.focus();
    }
  };

  return (
    <form
      className="composer"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <label htmlFor={id} className="sr-only">
        Your message
      </label>
      <div className="composer-controls">
        <textarea
          ref={inputRef}
          id={id}
          rows={2}
          value={draft}
          disabled={!enabled}
          autoComplete="off"
          aria-invalid={!!error || characters > MESSAGE_LIMIT}
          aria-describedby={`${id}-hint ${error ? `${id}-error` : ""}`.trim()}
          placeholder={
            enabled
              ? "Write a message..."
              : waiting
                ? "Waiting for a partner..."
                : "Start a conversation to send a message"
          }
          onChange={(event) => {
            setDraft(event.target.value);
            setError("");
            onTyping(event.target.value.trim().length > 0);
          }}
          onCompositionStart={() => {
            composingRef.current = true;
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
          }}
          onKeyDown={(event) => {
            if (
              event.key === "Enter" &&
              !event.shiftKey &&
              !event.nativeEvent.isComposing &&
              !composingRef.current
            ) {
              event.preventDefault();
              submit();
            }
          }}
        />
        <Button type="submit" disabled={!enabled || !draft.trim()}>
          Send<span aria-hidden="true"> &gt;</span>
        </Button>
      </div>
      <div className="composer-hints" id={`${id}-hint`}>
        <span>
          Enter to send <span aria-hidden="true">/</span> Shift+Enter for a new line
        </span>
        <span className={characters > MESSAGE_LIMIT ? "text-danger" : ""}>
          {characters.toLocaleString("en")} / 2,000
        </span>
      </div>
      {error && (
        <p className="composer-error" id={`${id}-error`} role="alert">
          {error}
        </p>
      )}
    </form>
  );
}
