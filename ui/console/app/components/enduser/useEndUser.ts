import type { AppView, SessionView, TokenView } from '#base/lib/account/views';
import type { ShownError } from '#base/lib/apperror/message';
import type { MethodRow } from '#base/lib/signin/methods';
import type { EndUser } from '#gen/enduser/v1/enduser_pb';
import { useConfirm } from '#base/composables/useConfirm';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { appView, sessionView, tokenView } from '#base/lib/account/views';
import { errorCode } from '#base/lib/apperror/message';
import { methodRows } from '#base/lib/signin/methods';

// useEndUser shows one end user to an admin and revokes their sessions, apps and tokens after a confirm.
export function useEndUser(projectId: string, endUserId: string) {
  const { $endUserClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const user = ref<EndUser | null>(null);
  const methods = ref<MethodRow[]>([]);
  const sessions = ref<SessionView[]>([]);
  const apps = ref<AppView[]>([]);
  const tokens = ref<TokenView[]>([]);
  const notFound = ref(false);
  const loading = ref(false);
  const loadError = ref<ShownError | null>(null);
  const ready = computed(() => user.value !== null && loadError.value === null);
  const ids = { projectId, endUserId };

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.getEndUser(ids);
      const now = new Date();
      user.value = res.endUser ?? null;
      notFound.value = !res.endUser;
      methods.value = methodRows(res.methods);
      sessions.value = res.sessions.map(sessionView);
      apps.value = res.apps.map(appView);
      tokens.value = res.tokens.map(t => tokenView(t, now));
    } catch (e) {
      notFound.value = errorCode(e) === 'EUS001';
      loadError.value = errorMessage(e, 'error.load_end_users');
    } finally {
      loading.value = false;
    }
  }

  const endSession = useConfirm<SessionView>(async (s) => {
    await client.revokeEndUserSession({ ...ids, sessionId: s.id });
    sessions.value = sessions.value.filter(x => x.id !== s.id);
  }, 'error.end_user_revoke');

  const disconnect = useConfirm<AppView>(async (app) => {
    await client.disconnectEndUserApp({ ...ids, clientId: app.clientId });
    apps.value = apps.value.filter(a => a.clientId !== app.clientId);
  }, 'error.end_user_revoke');

  const revokeToken = useConfirm<TokenView>(async (token) => {
    await client.revokeEndUserToken({ ...ids, tokenId: token.id });
    tokens.value = tokens.value.map(t => (t.id === token.id ? { ...t, status: 'revoked', revokedAt: new Date() } : t));
  }, 'error.end_user_revoke');

  return { user, methods, sessions, apps, tokens, notFound, loading, loadError, ready, load, endSession, disconnect, revokeToken };
}
