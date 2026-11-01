import { reactive } from 'vue';

export type ToastKind = 'success' | 'info' | 'error';

export interface Toast {
  id: number;
  kind: ToastKind;
  text: string;
}

const toasts = reactive<Toast[]>([]);
let next = 0;

// useToast shares one toast list per page without Pinia, so ui/mcp can use it too.
export function useToast() {
  function dismiss(id: number) {
    const i = toasts.findIndex(t => t.id === id);
    if (i >= 0) {
      toasts.splice(i, 1);
    }
  }

  function push(toast: Omit<Toast, 'id'>): number {
    const id = ++next;
    toasts.push({ ...toast, id });
    return id;
  }

  return { toasts, push, dismiss };
}
