import type { ShownError } from '#base/lib/apperror/message';
import type { EndUser } from '#gen/enduser/v1/enduser_pb';
import { useErrorMessage } from '#base/composables/useErrorMessage';

const pageSize = 50;

// useEndUserList pages through a project's end users, newest first, filtered by email.
export function useEndUserList(projectId: string) {
  const { $endUserClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const users = ref<EndUser[]>([]);
  const query = ref('');
  const next = ref('');
  const loading = ref(false);
  const loadError = ref<ShownError | null>(null);
  const shown = ref('');
  let latest = 0;

  async function fetch(filter: string, after: string): Promise<boolean> {
    const ticket = ++latest;
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.listEndUsers({ projectId, query: filter, limit: pageSize, after });
      if (ticket !== latest) {
        return false;
      }
      users.value = after ? [...users.value, ...res.endUsers] : res.endUsers;
      next.value = res.next;
      return true;
    } catch (e) {
      if (ticket === latest) {
        loadError.value = errorMessage(e, 'error.load_end_users');
      }
      return false;
    } finally {
      if (ticket === latest) {
        loading.value = false;
      }
    }
  }

  function load() {
    shown.value = query.value.trim();
    return fetch(shown.value, '');
  }

  function more() {
    return next.value ? fetch(shown.value, next.value) : Promise.resolve(false);
  }

  return { users, query, shown, next, loading, loadError, load, more };
}
