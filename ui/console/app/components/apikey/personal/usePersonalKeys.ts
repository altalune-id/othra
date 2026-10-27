import type { ShownError } from '#base/lib/apperror/message';
import type { APIKey } from '~/stores/apiKeys';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useScopeLabel } from '#base/composables/useScopeLabel';
import { toAPIKey } from '~/stores/apiKeys';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';

export interface PersonalKeyRow extends APIKey {
  orgName: string;
  scopeLabels: string[];
}

export interface CreatedKey {
  plaintext: string;
  snippet: string;
}

// createdKeyState carries a new key's secret from the create page to the list, once; it never reaches the URL.
export function createdKeyState() {
  return useState<CreatedKey | null>('apikeys.created', () => null);
}

function ts(k: APIKey): number {
  return k.createdAt?.getTime() ?? 0;
}

function newestFirst(a: APIKey, b: APIKey): number {
  return ts(b) - ts(a);
}

// NOTE: the org settings stores hold one org at a time, so this fans out per org into its own state.
export function usePersonalKeys() {
  const auth = useAuthStore();
  const orgs = useOrgsStore();
  const scopeLabel = useScopeLabel();
  const errorMessage = useErrorMessage();
  const { $apiKeyClient: client } = useNuxtApp();

  const byOrg = ref<Record<string, APIKey[]>>({});
  const failedIds = ref<string[]>([]);
  const loaded = ref(false);
  const error = ref<ShownError | null>(null);
  const created = createdKeyState();

  const signedIn = computed(() => auth.me?.source === 'local');
  const oneOrg = computed(() => orgs.items.length === 1);
  const failed = computed(() => orgs.items.filter(o => failedIds.value.includes(o.id)).map(o => ({ id: o.id, name: o.name })));
  const rows = computed<PersonalKeyRow[]>(() =>
    [...orgs.items]
      .sort((a, b) => a.name.localeCompare(b.name))
      .flatMap(org => [...(byOrg.value[org.id] ?? [])]
        .sort((a, b) => Number(!!a.revokedAt) - Number(!!b.revokedAt) || newestFirst(a, b))
        .map(k => ({ ...k, orgName: org.name, scopeLabels: k.scopes.map(scopeLabel) }))),
  );

  async function loadOrg(orgId: string) {
    try {
      const res = await client.listAPIKeys({ orgId });
      byOrg.value = { ...byOrg.value, [orgId]: (res.keys ?? []).filter(k => k.kind === 'personal').map(toAPIKey) };
      failedIds.value = failedIds.value.filter(id => id !== orgId);
    } catch {
      if (!failedIds.value.includes(orgId)) {
        failedIds.value = [...failedIds.value, orgId];
      }
    }
  }

  async function load() {
    await Promise.all([auth.load(), orgs.ensureLoaded()]);
    if (signedIn.value) {
      await Promise.allSettled(orgs.items.map(o => loadOrg(o.id)));
    }
    loaded.value = true;
  }

  async function revoke(orgId: string, hint: string) {
    error.value = null;
    try {
      await client.revokeAPIKey({ orgId, hint });
      const at = new Date();
      byOrg.value = {
        ...byOrg.value,
        [orgId]: (byOrg.value[orgId] ?? []).map(k => (!k.revokedAt && k.secretHint === hint ? { ...k, revokedAt: at } : k)),
      };
    } catch (e) {
      error.value = errorMessage(e, 'error.revoke_apikey');
    }
  }

  function dismiss() {
    created.value = null;
  }

  return { rows, failed, oneOrg, signedIn, loaded, error, created, load, retry: loadOrg, revoke, dismiss };
}
