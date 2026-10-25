import ChatActions from "../features/chat/components/chat-actions";
import ChatHeader from "../features/chat/components/chat-header";
import ChatStatus from "../features/chat/components/chat-status";
import MessageComposer from "../features/chat/components/message-composer";
import MessageList from "../features/chat/components/message-list";
import { canSendCommand } from "../features/chat/state";
import useChat from "../features/chat/hooks/use-chat";

export default function Chat() {
  const { state, join, cancel, send, stop, next, typing, retry } = useChat();
  const enabled = canSendCommand(state, "chat.send");

  return (
    <div className="page-shell">
      <a href="#conversation" className="skip-link">
        Skip to conversation
      </a>
      <ChatHeader />
      <main id="conversation" className="chat-panel">
        <ChatStatus state={state} />
        {state.error && (
          <p className="error-notice" role="alert">
            {state.error}
            {state.retryUntil > Date.now() ? " Please wait a moment before trying again." : ""}
          </p>
        )}
        <MessageList state={state} onStart={join} onCancel={cancel} onRetry={retry} />
        <div className="typing-status" aria-hidden="true">
          {state.peerTyping && state.chat === "matched" ? "Stranger is typing..." : ""}
        </div>
        <MessageComposer
          key={state.contentVersion}
          enabled={enabled}
          waiting={state.chat === "waiting"}
          onSend={send}
          onTyping={typing}
        />
        {state.chat === "matched" && <ChatActions state={state} onStop={stop} onNext={next} />}
      </main>
      <footer className="privacy-note">
        <p>
          Keep personal details private. Conversations are not saved by this app. User content is
          not moderated.
        </p>
        <details>
          <summary>About your privacy</summary>
          <p>
            Messages and drafts stay in memory and are cleared when a conversation ends or the
            connection is lost. This tab keeps up to 200 messages. The other person can copy or
            screenshot what you share.
          </p>
          <p>
            The server reads messages to relay them and processes network metadata, including IP
            addresses. This local HTTP/WS connection is not encrypted or end-to-end encrypted.
            Refreshing starts a new session.
          </p>
        </details>
      </footer>
    </div>
  );
}
