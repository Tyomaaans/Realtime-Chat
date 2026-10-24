import { useEffect, useId, useRef } from "react";
import Button from "./Button";

interface ConfirmDialogProps {
  action: "stop" | "next";
  onCancel: () => void;
  onConfirm: () => void;
}

export default function ConfirmDialog({ action, onCancel, onConfirm }: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const id = useId();

  useEffect(() => {
    const trigger = document.activeElement;
    const dialog = dialogRef.current;
    dialog?.showModal();
    return () => {
      dialog?.close();
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus();
    };
  }, []);

  return (
    <dialog
      ref={dialogRef}
      className="confirm-dialog"
      aria-labelledby={`${id}-title`}
      aria-describedby={`${id}-description`}
      onCancel={(event) => { event.preventDefault(); onCancel(); }}
    >
      <h2 id={`${id}-title`}>{action === "next" ? "Find someone new?" : "End this conversation?"}</h2>
      <p id={`${id}-description`}>This conversation and your draft will be cleared for both people. You cannot return to it.</p>
      <div className="dialog-actions">
        <Button variant="secondary" onClick={onCancel} autoFocus>Keep chatting</Button>
        <Button variant={action === "stop" ? "danger" : "primary"} onClick={onConfirm}>
          {action === "next" ? "Find someone new" : "End conversation"}
        </Button>
      </div>
    </dialog>
  );
}
