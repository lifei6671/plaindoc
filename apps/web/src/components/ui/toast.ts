import { toast } from "sonner";

export type ToastVariant = "success" | "info" | "error";

export function showToast(message: string, variant: ToastVariant = "error"): void {
  const normalizedMessage = message.trim();
  if (!normalizedMessage) {
    return;
  }

  if (variant === "success") {
    toast.success(normalizedMessage);
    return;
  }
  if (variant === "info") {
    toast.info(normalizedMessage);
    return;
  }
  toast.error(normalizedMessage);
}
