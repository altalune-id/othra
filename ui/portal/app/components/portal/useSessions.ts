import type { SessionView } from '#base/lib/account/views';
import type { ShownError } from '#base/lib/apperror/message';
import { useConfirm } from '#base/composables/useConfirm';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { sessionView } from '#base/lib/account/views';

// useSessions lists the end user's live sessions, current first, and ends one or every other after a confirm.
export function useSessions() {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const sessions = ref<SessionView[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);
  const others = computed(() => sessions.value.filter(s => !s.current));

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.listSessions({});
      sessions.value = res.sessions.map(sessionView).sort((a, b) => Number(b.current) - Number(a.current));
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_sessions');
    } finally {
      loading.value = false;
    }
  }

  const end = useConfirm<SessionView>(async (s) => {
    await client.endSession({ sessionId: s.id });
    sessions.value = sessions.value.filter(x => x.id !== s.id);
  }, 'error.end_session');

  const endOthers = useConfirm<true>(async () => {
    await client.endOtherSessions({});
    sessions.value = sessions.value.filter(x => x.current);
  }, 'error.end_session');

  return { sessions, others, loading, loadError, load, end, endOthers };
}
