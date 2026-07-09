import { useState, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

// ConfirmButton wraps a destructive/operational action in a confirmation dialog.
// It surfaces pending state and any error message returned by the action.
export function ConfirmButton({
  trigger,
  title,
  description,
  confirmText = "确认",
  onConfirm,
  pending = false,
  error,
}: {
  trigger: ReactNode;
  title: string;
  description: string;
  confirmText?: string;
  onConfirm: () => void;
  pending?: boolean;
  error?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={pending}>
            取消
          </Button>
          <Button onClick={onConfirm} disabled={pending}>
            {pending ? "正在处理..." : confirmText}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
