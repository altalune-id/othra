import type { AppView } from '#base/lib/account/views';
import type { ShownError } from '#base/lib/apperror/message';
import { useConfirm } from '#base/composables/useConfirm';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { appView } from '#base/lib/account/views';

// useConnectedApps lists the apps that can use the account and disconnects one after a confirm.
export function useConnectedApps() {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const apps = ref<AppView[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      apps.value = (await client.listConnectedApps({})).apps.map(appView);
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_apps');
    } finally {
      loading.value = false;
    }
  }

  const disconnect = useConfirm<AppView>(async (app) => {
    await client.disconnectApp({ clientId: app.clientId });
    apps.value = apps.value.filter(a => a.clientId !== app.clientId);
  }, 'error.disconnect_app');

  return { apps, loading, loadError, load, disconnect };
}
