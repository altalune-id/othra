import type { Ref } from 'vue';
import type { ShownError } from '../lib/apperror/message';
import { useErrorMessage } from './useErrorMessage';

export interface ConfirmHandle {
  target: Ref<unknown>;
  busy: Ref<boolean>;
  error: Ref<ShownError | null>;
  cancel: () => void;
  confirm: () => Promise<boolean>;
}

// useConfirm runs one destructive action on a target after a confirm, keeping the confirm open on a refusal.
export function useConfirm<T>(run: (target: T) => Promise<void>, fallbackKey: string) {
  const errorMessage = useErrorMessage();
  const target = ref<T | null>(null) as Ref<T | null>;
  const busy = ref(false);
  const error = ref<ShownError | null>(null);

  function ask(next: T) {
    error.value = null;
    target.value = next;
  }

  function cancel() {
    if (!busy.value) {
      target.value = null;
      error.value = null;
    }
  }

  async function confirm(): Promise<boolean> {
    const current = target.value;
    if (current === null || busy.value) {
      return false;
    }
    busy.value = true;
    error.value = null;
    try {
      await run(current);
      target.value = null;
      return true;
    } catch (e) {
      error.value = errorMessage(e, fallbackKey);
      return false;
    } finally {
      busy.value = false;
    }
  }

  return { target, busy, error, ask, cancel, confirm };
}
