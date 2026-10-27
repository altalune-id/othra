import type { ShownError } from '#base/lib/apperror/message';
import type { SigningKey } from '#gen/keys/v1/keys_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { formatDate } from '#base/lib/format/date';
import { SigningKeyStatus } from '#gen/keys/v1/keys_pb';

const statusOrder: Record<SigningKeyStatus, number> = {
  [SigningKeyStatus.NEXT]: 0,
  [SigningKeyStatus.ACTIVE]: 1,
  [SigningKeyStatus.RETIRING]: 2,
  [SigningKeyStatus.RETIRED]: 3,
  [SigningKeyStatus.UNSPECIFIED]: 4,
};
const takeoverGraceMs = 70_000;
const retryEveryMs = 30_000;
const maxRetries = 5;

export interface RotatedKey {
  kid: string;
  activatesAt: Date | null;
}

// shortKid is the first 8 characters of a key ID, as the console shows it.
export function shortKid(kid: string): string {
  return kid.slice(0, 8);
}

// formatWhen shows only the time for a moment later today, and the date with the time otherwise.
export function formatWhen(when: Date, locale: string, now: Date = new Date()): string {
  const sameDay = when.toDateString() === now.toDateString();
  return formatDate(when, locale, { style: sameDay ? 'time' : 'moment' });
}

function millis(key: SigningKey): number {
  return key.createdAt ? timestampDate(key.createdAt).getTime() : 0;
}

// useSigningKeys lists a project's signing keys, rotates them, and refetches when a waiting key should have taken over.
export function useSigningKeys(projectId: string) {
  const { $signingKeyClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();

  const keys = ref<SigningKey[]>([]);
  const loading = ref(true);
  const loaded = ref(false);
  const loadError = ref<ShownError | null>(null);
  const confirming = ref(false);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const rotated = ref<RotatedKey | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let retries = 0;
  let disposed = false;

  const waiting = computed(() => keys.value.find(k => k.status === SigningKeyStatus.NEXT));
  const waitingUntil = computed(() => (waiting.value?.activatesAt ? timestampDate(waiting.value.activatesAt) : null));
  const noActive = computed(() => loaded.value && !keys.value.some(k => k.status === SigningKeyStatus.ACTIVE));
  const hasRetired = computed(() => keys.value.some(k => k.status === SigningKeyStatus.RETIRED));

  function schedule(first: boolean) {
    clearTimeout(timer);
    timer = undefined;
    const until = waitingUntil.value;
    if (disposed || !until) {
      retries = 0;
      return;
    }
    if (!first && retries >= maxRetries) {
      return;
    }
    const delay = first ? Math.max(retryEveryMs, until.getTime() + takeoverGraceMs - Date.now()) : retryEveryMs;
    timer = setTimeout(() => {
      if (!first) {
        retries++;
      }
      void fetch(false);
    }, delay);
  }

  async function fetch(first: boolean) {
    try {
      const res = await client.listSigningKeys({ projectId });
      keys.value = [...res.keys].sort((a, b) => statusOrder[a.status] - statusOrder[b.status] || millis(b) - millis(a));
      loaded.value = true;
      loadError.value = null;
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_keys');
    } finally {
      loading.value = false;
    }
    if (first) {
      retries = 0;
    }
    schedule(first);
  }

  async function load() {
    loading.value = !loaded.value;
    await fetch(true);
  }

  function askRotate() {
    error.value = null;
    confirming.value = true;
  }

  async function cancelRotate() {
    const refused = error.value !== null;
    confirming.value = false;
    error.value = null;
    if (refused) {
      await load();
    }
  }

  async function confirmRotate() {
    if (!confirming.value || busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.rotateSigningKey({ projectId, immediate: false });
      const key = res.key;
      rotated.value = key ? { kid: key.kid, activatesAt: key.activatesAt ? timestampDate(key.activatesAt) : null } : null;
      confirming.value = false;
      await load();
    } catch (e) {
      error.value = errorMessage(e, 'error.rotate_key');
    } finally {
      busy.value = false;
    }
  }

  function dispose() {
    disposed = true;
    clearTimeout(timer);
  }

  return { keys, loading, loaded, loadError, confirming, busy, error, rotated, waitingUntil, noActive, hasRetired, load, askRotate, cancelRotate, confirmRotate, dispose };
}
