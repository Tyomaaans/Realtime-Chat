import { useState } from "react";
import type { ChatStateData } from "../types/chat";
import Button from "./ui/Button";
import ConfirmDialog from "./ui/ConfirmDialog";

export default function ChatActions({ state, onStop, onNext }: { state: ChatStateData; onStop: () => void; onNext: () => void }) {
  const [dialog, setDialog] = useState<"stop" | "next" | null>(null);
  const disabled = state.pending !== null || Date.now() < state.retryUntil;

  return (
    <div className="chat-actions">
      <p>You can leave a conversation at any time.</p>
      <div>
        <Button variant="quiet" disabled={disabled} onClick={() => setDialog("stop")}>Stop</Button>
        <Button variant="secondary" disabled={disabled} onClick={() => setDialog("next")}>Next person <span aria-hidden="true">&gt;</span></Button>
      </div>
      {dialog && <ConfirmDialog action={dialog} onCancel={() => setDialog(null)} onConfirm={() => {
        if (dialog === "stop") onStop();
        else onNext();
        setDialog(null);
      }} />}
    </div>
  );
}
