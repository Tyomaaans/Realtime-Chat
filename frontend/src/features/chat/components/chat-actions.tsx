import { useState } from "react";
import { isChatBusy } from "../state";
import type { ChatSessionState } from "../types";
import Button from "../../../components/ui/button";
import ConfirmDialog from "./confirm-dialog";

interface ChatActionsProps {
  state: ChatSessionState;
  onStop: () => void;
  onNext: () => void;
}

export default function ChatActions({ state, onStop, onNext }: ChatActionsProps) {
  const [dialog, setDialog] = useState<"stop" | "next" | null>(null);
  const disabled = isChatBusy(state);

  return (
    <div className="chat-actions">
      <p>You can leave a conversation at any time.</p>
      <div>
        <Button variant="quiet" disabled={disabled} onClick={() => setDialog("stop")}>
          Stop
        </Button>
        <Button variant="secondary" disabled={disabled} onClick={() => setDialog("next")}>
          Next person <span aria-hidden="true">&gt;</span>
        </Button>
      </div>
      {dialog && (
        <ConfirmDialog
          action={dialog}
          onCancel={() => setDialog(null)}
          onConfirm={() => {
            if (dialog === "stop") {
              onStop();
            } else {
              onNext();
            }

            setDialog(null);
          }}
        />
      )}
    </div>
  );
}
