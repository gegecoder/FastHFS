import { readonly, ref } from "vue";

export type ToastType = "success" | "error" | "info";

export interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
}

const items = ref<ToastItem[]>([]);
const timers = new Map<number, number>();
let sequence = 0;

function dismiss(id: number): void {
  const timer = timers.get(id);
  if (timer !== undefined) {
    window.clearTimeout(timer);
    timers.delete(id);
  }
  items.value = items.value.filter((item) => item.id !== id);
}

function push(message: string, type: ToastType, duration = 3200): number {
  const id = ++sequence;
  items.value = [...items.value, { id, type, message }];
  if (items.value.length > 5) {
    const oldest = items.value[0];
    if (oldest) {
      dismiss(oldest.id);
    }
  }
  if (duration > 0) {
    timers.set(
      id,
      window.setTimeout(() => dismiss(id), duration),
    );
  }
  return id;
}

export function useToast() {
  return {
    toasts: readonly(items),
    dismiss,
    toast: (message: string, type: ToastType = "info", duration = 3200) => push(message, type, duration),
    success: (message: string, duration = 2800) => push(message, "success", duration),
    error: (message: string, duration = 4200) => push(message, "error", duration),
    info: (message: string, duration = 3200) => push(message, "info", duration),
  };
}
