import { ConfirmDialog } from "../ConfirmDialog";

/* ─── Props ─── */

interface DirtyGuardProps {
  isOpen: boolean;
  onDiscard: () => void;
  onStay: () => void;
}

/* ─── Component ─── */

/**
 * Confirmation dialog shown when the user tries to switch tabs
 * while the active tab has unsaved changes.
 *
 * - "Discard" confirms the switch (resets dirty state, switches tab)
 * - "Stay" cancels the switch (remains on current tab)
 */
export function DirtyGuard({ isOpen, onDiscard, onStay }: DirtyGuardProps) {
  return (
    <ConfirmDialog
      open={isOpen}
      title="Unsaved Changes"
      message="You have unsaved changes. Discard them?"
      confirmLabel="Discard"
      cancelLabel="Stay"
      confirmVariant="danger"
      onConfirm={onDiscard}
      onCancel={onStay}
    />
  );
}
