import type { TokenView } from '#base/lib/account/views';
import type { ShownError } from '#base/lib/apperror/message';
import { useConfirm } from '#base/composables/useConfirm';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { tokenView } from '#base/lib/account/views';

// usePersonalTokens lists the end user's personal tokens and revokes an active one after a confirm.
export function usePersonalTokens() {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const tokens = ref<TokenView[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const now = new Date();
      tokens.value = (await client.listPersonalTokens({})).tokens.map(t => tokenView(t, now));
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_tokens');
    } finally {
      loading.value = false;
    }
  }

  const revoke = useConfirm<TokenView>(async (token) => {
    await client.revokePersonalToken({ tokenId: token.id });
    tokens.value = tokens.value.map(t => (t.id === token.id ? { ...t, status: 'revoked', revokedAt: new Date() } : t));
  }, 'error.revoke_token');

  return { tokens, loading, loadError, load, revoke };
}
