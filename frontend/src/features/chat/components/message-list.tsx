import { useEffect, useRef, useState } from "react";
import { isChatBusy } from "../state";
import type { ChatSessionState } from "../types";
import Button from "../../../components/ui/button";

interface MessageListProps {
  state: ChatSessionState;
  onStart: () => void;
  onCancel: () => void;
  onRetry: () => void;
}

export default function MessageList({ state, onStart, onCancel, onRetry }: MessageListProps) {
  const logRef = useRef<HTMLDivElement>(null);
  const nearBottomRef = useRef(true);
  const [newMessages, setNewMessages] = useState(false);
  const online = state.connection === "open";
  const busy = isChatBusy(state);

  useEffect(() => {
    nearBottomRef.current = true;
    setNewMessages(false);
  }, [state.contentVersion]);

  useEffect(() => {
    const log = logRef.current;
    if (!log) {
      return;
    }

    if (nearBottomRef.current) {
      log.scrollTop = log.scrollHeight;
      setNewMessages(false);
    } else if (state.messages.length) {
      setNewMessages(true);
    }
  }, [state.messages]);

  return (
    <div className="transcript-wrap">
      <div
        ref={logRef}
        className="transcript"
        role="log"
        aria-label="Conversation messages"
        aria-live="polite"
        aria-relevant="additions"
        tabIndex={0}
        onScroll={() => {
          const log = logRef.current;
          if (!log) {
            return;
          }

          nearBottomRef.current = log.scrollHeight - log.scrollTop - log.clientHeight < 80;
          if (nearBottomRef.current) {
            setNewMessages(false);
          }
        }}
      >
        {state.chat !== "matched" && (
          <div className="empty-state">
            <div className="conversation-symbol" aria-hidden="true">
              <svg viewBox="0 0 64 64" width="64" height="64" fill="none">
                <path
                  d="M12 15h33v24H27L16 47v-8h-4V15Z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinejoin="round"
                />
                <path
                  d="M45 25h8v25h-4v7l-10-7H28v-7M21 24h15m-15 7h9"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
            <span className="eyebrow">ANONYMOUS TEXT CHAT</span>
            <h2>
              {state.chat === "waiting"
                ? "Waiting for a partner"
                : state.connection === "closed"
                  ? "Connection unavailable"
                  : "Chat with someone new"}
            </h2>
            <p>
              {state.chat === "waiting"
                ? "Leave this tab open while we look. You can cancel whenever you like."
                : "Meet a stranger for a one-to-one conversation. No account, no introductions required."}
            </p>
            {state.notice && (
              <p className="session-notice" role="status">
                {state.notice}
              </p>
            )}
            {state.connection === "closed" ? (
              <Button onClick={onRetry}>Retry connection</Button>
            ) : state.chat === "waiting" ? (
              <Button variant="secondary" disabled={busy} onClick={onCancel}>
                Cancel search
              </Button>
            ) : (
              <Button disabled={!online || busy} onClick={onStart}>
                {state.notice ? "Find someone" : online ? "Start chatting" : "Connecting..."}
              </Button>
            )}
            <span className="empty-caption">No account required.</span>
          </div>
        )}
        {state.chat === "matched" && state.messages.length === 0 && (
          <div className="conversation-start">
            <span className="status-dot" aria-hidden="true" />
            <p>You're connected. Say hello.</p>
          </div>
        )}
        {state.removed && (
          <p className="retention-notice">
            Earlier messages were removed. Only the latest 200 messages are kept in this tab.
          </p>
        )}
        {state.messages.map((message) => (
          <article key={message.message_id} className={`message message-${message.sender}`}>
            <span className="message-label">{message.sender === "self" ? "You" : "Stranger"}</span>
            <p>{message.text}</p>
            <time dateTime={message.sent_at}>
              {new Date(message.sent_at).toLocaleTimeString("en", {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </time>
          </article>
        ))}
      </div>
      {newMessages && state.chat === "matched" && (
        <Button
          className="new-messages"
          variant="secondary"
          onClick={() => {
            const log = logRef.current;
            if (log) {
              log.scrollTop = log.scrollHeight;
            }

            nearBottomRef.current = true;
            setNewMessages(false);
          }}
        >
          New messages
        </Button>
      )}
    </div>
  );
}
