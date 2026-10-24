import type { ChatStateData } from "../types/chat";

export default function ChatStatus({ state }: { state: ChatStateData }) {
  let title = "Your next conversation";
  let description = "A little space to meet someone new.";
  let label = "Connecting";
  if (state.connection === "reconnecting") {
    title = "Connection interrupted";
    description = `Reconnecting, attempt ${state.attempt} of 5. Your previous conversation has ended.`;
    label = "Reconnecting";
  } else if (state.connection === "closed") {
    title = "You're offline";
    description = "The server may be full or unavailable. Check your connection and select Retry.";
    label = "Offline";
  } else if (state.connection === "open") {
    label = "Ready";
    if (state.chat === "waiting") {
      title = "Looking for someone";
      description = "We'll connect you as soon as a partner is available.";
      label = "Searching";
    } else if (state.chat === "matched") {
      title = "You're chatting with a stranger";
      description = "Be kind, stay curious, and keep personal details private.";
      label = "Connected";
    }
  }

  return (
    <div className="chat-status">
      <div className="status-copy" role="status" aria-live="polite" aria-atomic="true">
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      <span className={`status-label ${state.connection === "open" ? "available" : ""}`}>
        <span className={`status-dot ${state.chat === "waiting" ? "searching" : ""}`} aria-hidden="true" />{label}
      </span>
    </div>
  );
}
