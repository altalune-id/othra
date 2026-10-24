import type { Client } from '~~/gen/client/v1/client_pb';
import type { ShownError } from '~/lib/apperror/message';
import { ClientRegistration } from '~~/gen/client/v1/client_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { errorCode } from '~/lib/apperror/message';

// useClientList loads a project's OAuth clients.
export function useClientList(projectId: string) {
  const { $clientClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const clients = ref<Client[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.listClients({ projectId });
      clients.value = res.clients.filter(c => c.registration !== ClientRegistration.CIMD);
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_clients');
    } finally {
      loading.value = false;
    }
  }

  return { clients, loading, loadError, load };
}

// useClient loads one OAuth client and deletes it after a confirm.
export function useClient(projectId: string, clientId: string) {
  const { $clientClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const current = ref<Client | null>(null);
  const notFound = ref(false);
  const loadError = ref<ShownError | null>(null);
  const deleting = ref(false);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);

  async function load() {
    loadError.value = null;
    try {
      const res = await client.getClient({ projectId, clientId });
      current.value = res.client ?? null;
      notFound.value = !res.client;
    } catch (e) {
      notFound.value = errorCode(e) === 'CLT001';
      loadError.value = errorMessage(e, 'error.load_clients');
    }
  }

  function replace(next: Client) {
    current.value = next;
  }

  function askDelete() {
    error.value = null;
    deleting.value = true;
  }

  function cancelDelete() {
    if (!busy.value) {
      deleting.value = false;
      error.value = null;
    }
  }

  async function confirmDelete(): Promise<boolean> {
    if (!deleting.value || busy.value) {
      return false;
    }
    busy.value = true;
    error.value = null;
    try {
      await client.deleteClient({ projectId, clientId });
      deleting.value = false;
      return true;
    } catch (e) {
      error.value = errorMessage(e, 'error.delete_client');
      return false;
    } finally {
      busy.value = false;
    }
  }

  return { client: current, notFound, loadError, deleting, busy, error, load, replace, askDelete, cancelDelete, confirmDelete };
}
