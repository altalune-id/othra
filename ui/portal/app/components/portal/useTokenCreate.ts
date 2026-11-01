import type { ShownError } from '#base/lib/apperror/message';
import type { TokenServer } from '#gen/account/v1/account_pb';
import { create } from '@bufbuild/protobuf';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useValidation } from '#base/composables/useValidation';
import { errorCode } from '#base/lib/apperror/message';
import { mcpSnippet } from '#base/lib/tokens/mcpSnippet';
import { CreatePersonalTokenRequestSchema, TokenExpiry } from '#gen/account/v1/account_pb';

export const expiryOptions = [
  { value: TokenExpiry.TOKEN_EXPIRY_7_DAYS, days: 7 },
  { value: TokenExpiry.TOKEN_EXPIRY_30_DAYS, days: 30 },
  { value: TokenExpiry.TOKEN_EXPIRY_90_DAYS, days: 90 },
];

const draftKey = 'othra.portal.token-draft';

interface Draft {
  name: string;
  serverId: string;
  scopes: string[];
  expiry: TokenExpiry;
}

function readDraft(): Draft | null {
  try {
    const raw = sessionStorage.getItem(draftKey);
    sessionStorage.removeItem(draftKey);
    return raw ? JSON.parse(raw) as Draft : null;
  } catch {
    return null;
  }
}

function writeDraft(draft: Draft) {
  try {
    sessionStorage.setItem(draftKey, JSON.stringify(draft));
  } catch {
    // NOTE: without session storage the form comes back empty after the fresh sign-in.
  }
}

// useTokenCreate creates a personal token for one server and holds its secret until the person leaves the reveal.
export function useTokenCreate() {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const validation = useValidation(CreatePersonalTokenRequestSchema, { fields: ['name', 'server_id', 'scopes'], codes: { ACC006: 'server_id' } });
  const servers = ref<TokenServer[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);
  const name = ref('');
  const serverId = ref('');
  const scopes = ref<string[]>([]);
  const expiry = ref<TokenExpiry>(TokenExpiry.TOKEN_EXPIRY_30_DAYS);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const secret = ref('');
  const reauth = ref(false);
  const server = computed(() => servers.value.find(s => s.id === serverId.value) ?? null);
  const snippet = computed(() => (server.value && secret.value ? mcpSnippet(server.value.name || 'server', server.value.identifier, secret.value) : ''));

  watch(serverId, () => {
    scopes.value = [];
  }, { flush: 'sync' });

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      servers.value = (await client.listTokenServers({})).servers;
      if (servers.value.length === 1) {
        serverId.value = servers.value[0]!.id;
      }
      const draft = readDraft();
      if (draft && servers.value.some(s => s.id === draft.serverId)) {
        name.value = draft.name;
        serverId.value = draft.serverId;
        scopes.value = draft.scopes;
        expiry.value = draft.expiry;
      }
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_servers');
    } finally {
      loading.value = false;
    }
  }

  async function submit(): Promise<boolean> {
    if (busy.value) {
      return false;
    }
    error.value = null;
    reauth.value = false;
    const req = { name: name.value.trim(), serverId: serverId.value, scopes: [...scopes.value], expiry: expiry.value };
    if (!validation.check(create(CreatePersonalTokenRequestSchema, req))) {
      error.value = validation.formError.value;
      return false;
    }
    busy.value = true;
    try {
      const res = await client.createPersonalToken(req);
      secret.value = res.secret;
      return true;
    } catch (e) {
      reauth.value = errorCode(e) === 'ACC009';
      if (reauth.value) {
        writeDraft(req);
      }
      error.value = validation.fromServer(e, 'error.create_token');
      return false;
    } finally {
      busy.value = false;
    }
  }

  function forget() {
    secret.value = '';
  }

  return { servers, server, loading, loadError, name, serverId, scopes, expiry, busy, error, fieldErrors: validation.errors, secret, snippet, reauth, load, submit, forget };
}
