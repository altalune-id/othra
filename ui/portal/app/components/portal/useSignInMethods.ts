import type { ShownError } from '#base/lib/apperror/message';
import type { MethodRow } from '#base/lib/signin/methods';
import { useConfirm } from '#base/composables/useConfirm';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { methodRows } from '#base/lib/signin/methods';

// useSignInMethods lists the end user's sign-in methods and disconnects an upstream IdP after a confirm.
export function useSignInMethods() {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const rows = ref<MethodRow[]>([]);
  const linkToken = ref('');
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);
  const unlinked = ref(false);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.getSignInMethods({});
      rows.value = methodRows(res.methods);
      linkToken.value = res.linkToken;
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_methods');
    } finally {
      loading.value = false;
    }
  }

  const unlink = useConfirm<MethodRow>(async (row) => {
    await client.unlinkIdentity({ identityId: row.identityId });
    unlinked.value = true;
    await load();
  }, 'error.unlink');

  function askUnlink(row: MethodRow) {
    if (!row.canRemove) {
      return;
    }
    unlinked.value = false;
    unlink.ask(row);
  }

  return { rows, linkToken, loading, loadError, unlinked, load, unlink, askUnlink };
}
