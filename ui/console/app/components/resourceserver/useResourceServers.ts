import type { ShownError } from '#base/lib/apperror/message';
import type { ResourceServer, VerificationFailure } from '#gen/resourceserver/v1/resourceserver_pb';
import type { ScopeList } from './scopes';
import { create } from '@bufbuild/protobuf';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useValidation } from '#base/composables/useValidation';
import { errorCode, errorMeta } from '#base/lib/apperror/message';
import { CreateResourceServerRequestSchema, UpdateResourceServerRequestSchema } from '#gen/resourceserver/v1/resourceserver_pb';
import { identifierProblem, isHttpIdentifier } from './resourceUri';
import { checkScopes, maxDescription, maxScopes, scopeMessageKey } from './scopes';

// useResourceServerList loads a project's resource servers.
export function useResourceServerList(projectId: string) {
  const { $resourceServerClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const servers = ref<ResourceServer[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.listResourceServers({ projectId });
      servers.value = [...res.resourceServers];
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_rs');
    } finally {
      loading.value = false;
    }
  }

  return { servers, loading, loadError, load };
}

// useScopeRows edits a resource server's scope list and places refusals on the row they name.
export function useScopeRows(initial: string[], initialDescriptions: Record<string, string> = {}) {
  const { t } = useI18n();
  const rows = ref<string[]>(initial.length > 0 ? [...initial] : ['']);
  const descriptions = ref<string[]>(rows.value.map(s => initialDescriptions[s] ?? ''));
  const errors = ref<string[]>([]);
  const full = computed(() => rows.value.length >= maxScopes);

  function reset(next: string[], nextDescriptions: Record<string, string> = {}) {
    rows.value = next.length > 0 ? [...next] : [''];
    descriptions.value = rows.value.map(s => nextDescriptions[s] ?? '');
    errors.value = [];
  }

  function add() {
    if (!full.value) {
      rows.value = [...rows.value, ''];
      descriptions.value = [...descriptions.value, ''];
    }
  }

  function remove(index: number) {
    const next = rows.value.filter((_, i) => i !== index);
    const nextDescriptions = descriptions.value.filter((_, i) => i !== index);
    rows.value = next.length > 0 ? next : [''];
    descriptions.value = next.length > 0 ? nextDescriptions : [''];
    errors.value = errors.value.filter((_, i) => i !== index);
  }

  function describe(list: ScopeList): Record<string, string> {
    const out: Record<string, string> = {};
    list.scopes.forEach((scope, i) => {
      const text = (descriptions.value[list.rows[i]!] ?? '').trim();
      if (text) {
        out[scope] = text;
      }
    });
    return out;
  }

  function clearRow(index: number) {
    if (!errors.value[index]) {
      return;
    }
    const next = [...errors.value];
    next[index] = '';
    errors.value = next;
  }

  function check(): ScopeList | null {
    const list = checkScopes(rows.value);
    errors.value = list.problems.map((p, i) => {
      if (p) {
        return t(scopeMessageKey(p));
      }
      const text = (descriptions.value[i] ?? '').trim();
      return text.length > maxDescription ? t('apperror.RSV004_description', { scope: rows.value[i]!.trim() }) : '';
    });
    return errors.value.every(e => e === '') ? list : null;
  }

  function place(err: unknown, message: string, list: ScopeList): boolean {
    if (errorCode(err) !== 'RSV004') {
      return false;
    }
    const at = list.scopes.indexOf(errorMeta(err).scope ?? '');
    if (at < 0) {
      return false;
    }
    const next = [...errors.value];
    next[list.rows[at]!] = message;
    errors.value = next;
    return true;
  }

  return { rows, descriptions, errors, full, reset, add, remove, clearRow, check, place, describe };
}

// useResourceServerCreate registers a resource server with its name, identifier and scopes.
export function useResourceServerCreate(projectId: string) {
  const { $resourceServerClient: client } = useNuxtApp();
  const { t } = useI18n();
  const errorMessage = useErrorMessage();
  const validation = useValidation(CreateResourceServerRequestSchema, {
    fields: ['name', 'identifier', 'scopes'],
    codes: { RSV002: 'identifier', RSV003: 'identifier', RSV007: 'identifier' },
  });
  const name = ref('');
  const identifier = ref('');
  const scopes = useScopeRows([]);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const identifierHint = ref('');
  const trustAvailable = ref(false);
  const trustPicked = ref(false);
  const trustRequired = computed(() => isHttpIdentifier(identifier.value));
  const trusted = computed(() => trustAvailable.value && (trustRequired.value || trustPicked.value));

  async function loadTrust() {
    try {
      const res = await client.listResourceServers({ projectId });
      trustAvailable.value = res.trustAvailable;
    } catch {
      trustAvailable.value = false;
    }
  }

  const errors = computed(() => ({
    name: validation.errors.value.name ?? '',
    identifier: identifierHint.value || validation.errors.value.identifier || '',
  }));

  function clearIdentifier() {
    identifierHint.value = '';
  }

  function checkIdentifier() {
    const raw = identifier.value.trim();
    const problem = raw ? identifierProblem(raw) : null;
    identifierHint.value = problem ? t(`apperror.RSV002_${problem.reason}`, problem.canonical ? { canonical: problem.canonical } : {}) : '';
    return !problem;
  }

  async function submit(): Promise<ResourceServer | null> {
    if (busy.value) {
      return null;
    }
    error.value = null;
    const identifierOk = checkIdentifier();
    const list = scopes.check();
    const req = {
      projectId,
      name: name.value.trim(),
      identifier: identifier.value.trim(),
      scopes: list?.scopes ?? [],
      scopeDescriptions: list ? scopes.describe(list) : {},
      trusted: trusted.value,
    };
    const valid = validation.check(create(CreateResourceServerRequestSchema, req));
    if (!valid || !identifierOk || !list) {
      error.value = list ? validation.formError.value : null;
      return null;
    }
    busy.value = true;
    try {
      const res = await client.createResourceServer(req);
      return res.resourceServer ?? null;
    } catch (e) {
      if (!scopes.place(e, errorMessage(e, 'error.save_rs').message, list)) {
        error.value = validation.fromServer(e, 'error.save_rs');
      }
      return null;
    } finally {
      busy.value = false;
    }
  }

  return { name, identifier, scopes, busy, error, errors, trustAvailable, trustPicked, trustRequired, trusted, loadTrust, clearIdentifier, submit };
}

export interface VerifyOutcome {
  passed: boolean;
  failure: VerificationFailure | null;
}

// useResourceServer loads one resource server and runs its name, scopes, verification, trust and delete actions.
export function useResourceServer(projectId: string, resourceServerId: string) {
  const { $resourceServerClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const validation = useValidation(UpdateResourceServerRequestSchema, { fields: ['name', 'scopes'] });

  const server = ref<ResourceServer | null>(null);
  const notFound = ref(false);
  const loadError = ref<ShownError | null>(null);
  const name = ref('');
  const scopes = useScopeRows([]);
  const saving = ref<'' | 'name' | 'scopes'>('');
  const saved = ref<'' | 'name' | 'scopes'>('');
  const saveError = ref<{ part: 'name' | 'scopes', error: ShownError } | null>(null);
  const verifying = ref(false);
  const verified = ref<VerifyOutcome | null>(null);
  const verifyError = ref<ShownError | null>(null);
  const trustAsking = ref(false);
  const trustBusy = ref(false);
  const trustError = ref<ShownError | null>(null);
  const deleting = ref(false);
  const deleteBusy = ref(false);
  const deleteError = ref<ShownError | null>(null);

  const nameError = computed(() => validation.errors.value.name ?? '');

  function replace(next: ResourceServer | undefined) {
    if (!next) {
      return;
    }
    server.value = next;
    name.value = next.name;
    scopes.reset(next.scopes, next.scopeDescriptions);
  }

  async function load() {
    loadError.value = null;
    try {
      const res = await client.getResourceServer({ projectId, resourceServerId });
      notFound.value = !res.resourceServer;
      replace(res.resourceServer);
    } catch (e) {
      notFound.value = errorCode(e) === 'RSV001';
      loadError.value = errorMessage(e, 'error.load_rs');
    }
  }

  async function save(part: 'name' | 'scopes') {
    const current = server.value;
    if (!current || saving.value) {
      return;
    }
    saved.value = '';
    saveError.value = null;
    const list = part === 'scopes' ? scopes.check() : { scopes: [...current.scopes], rows: [], problems: [] };
    if (!list) {
      return;
    }
    const req = {
      projectId,
      resourceServerId,
      name: part === 'name' ? name.value.trim() : current.name,
      scopes: list.scopes,
      scopeDescriptions: part === 'scopes' ? scopes.describe(list) : { ...current.scopeDescriptions },
    };
    if (!validation.check(create(UpdateResourceServerRequestSchema, req))) {
      const formError = validation.formError.value;
      if (formError) {
        saveError.value = { part, error: formError };
      }
      return;
    }
    saving.value = part;
    try {
      const res = await client.updateResourceServer(req);
      const draft = { name: name.value, rows: scopes.rows.value, descriptions: scopes.descriptions.value };
      replace(res.resourceServer);
      if (part === 'name') {
        scopes.rows.value = draft.rows;
        scopes.descriptions.value = draft.descriptions;
      } else {
        name.value = draft.name;
      }
      saved.value = part;
    } catch (e) {
      const message = errorMessage(e, 'error.save_rs');
      if (part === 'name' || !scopes.place(e, message.message, list)) {
        const placed = validation.fromServer(e, 'error.save_rs');
        if (placed) {
          saveError.value = { part, error: placed };
        }
      }
    } finally {
      saving.value = '';
    }
  }

  async function verify() {
    if (verifying.value) {
      return;
    }
    verifying.value = true;
    verified.value = null;
    verifyError.value = null;
    try {
      const res = await client.verifyResourceServer({ projectId, resourceServerId });
      replace(res.resourceServer);
      verified.value = { passed: res.passed, failure: res.error ?? null };
    } catch (e) {
      verifyError.value = errorMessage(e, 'error.verify_rs');
    } finally {
      verifying.value = false;
    }
  }

  async function setTrust(trusted: boolean) {
    if (trustBusy.value) {
      return;
    }
    trustBusy.value = true;
    trustError.value = null;
    try {
      const res = await client.setResourceServerTrust({ projectId, resourceServerId, trusted });
      replace(res.resourceServer);
      trustAsking.value = false;
      verified.value = null;
    } catch (e) {
      trustError.value = errorMessage(e, 'error.trust_rs');
    } finally {
      trustBusy.value = false;
    }
  }

  async function toggleTrust() {
    if (server.value?.trusted) {
      await setTrust(false);
    } else {
      trustError.value = null;
      trustAsking.value = true;
    }
  }

  function cancelTrust() {
    if (!trustBusy.value) {
      trustAsking.value = false;
      trustError.value = null;
    }
  }

  function askDelete() {
    deleteError.value = null;
    deleting.value = true;
  }

  function cancelDelete() {
    if (!deleteBusy.value) {
      deleting.value = false;
      deleteError.value = null;
    }
  }

  async function confirmDelete(): Promise<boolean> {
    if (!deleting.value || deleteBusy.value) {
      return false;
    }
    deleteBusy.value = true;
    deleteError.value = null;
    try {
      await client.deleteResourceServer({ projectId, resourceServerId });
      deleting.value = false;
      return true;
    } catch (e) {
      deleteError.value = errorMessage(e, 'error.delete_rs');
      return false;
    } finally {
      deleteBusy.value = false;
    }
  }

  return {
    server,
    notFound,
    loadError,
    name,
    nameError,
    scopes,
    saving,
    saved,
    saveError,
    verifying,
    verified,
    verifyError,
    trustAsking,
    trustBusy,
    trustError,
    deleting,
    deleteBusy,
    deleteError,
    load,
    replace,
    save,
    verify,
    toggleTrust,
    setTrust,
    cancelTrust,
    askDelete,
    cancelDelete,
    confirmDelete,
  };
}
