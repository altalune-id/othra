import type { ShownError } from '~/lib/apperror/message';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import scopes from '@shared/scope/scopes.json';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useScopeLabel } from '~/composables/useScopeLabel';
import { mcpURL } from '~/lib/auth/urls';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { mintableForMe } from './grants';
import { mcpSnippet } from './mcpSnippet';
import { createdKeyState } from './usePersonalKeys';

export const expiryOptions = [
  { days: 7, labelKey: 'apikeys.days_7' },
  { days: 30, labelKey: 'apikeys.days_30' },
  { days: 90, labelKey: 'apikeys.days_90' },
  { days: 365, labelKey: 'apikeys.days_365' },
];

export function usePersonalKeyCreate() {
  const auth = useAuthStore();
  const orgs = useOrgsStore();
  const scopeLabel = useScopeLabel();
  const errorMessage = useErrorMessage();
  const { $apiKeyClient: client, $projectClient: projectClient } = useNuxtApp();
  const apiUrl = useRuntimeConfig().public.apiUrl;

  const name = ref('');
  const orgId = ref('');
  const expiresInDays = ref(30);
  const allProjects = ref(true);
  const projectIds = ref<string[]>([]);
  const scopesPicked = ref<string[]>([]);
  const projects = ref<{ id: string, name: string }[]>([]);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const projectCache = new Map<string, { id: string, name: string }[]>();

  const oneOrg = computed(() => orgs.items.length === 1);
  const orgOptions = computed(() => orgs.items.map(o => ({ value: o.id, label: o.name })));
  const orgName = computed(() => orgs.items.find(o => o.id === orgId.value)?.name ?? '');
  const canSubmit = computed(() => !busy.value && !!orgId.value && !!name.value.trim());
  const mintable = computed(() => {
    const org = orgs.items.find(o => o.id === orgId.value);
    if (!org) {
      return [];
    }
    const allowed = org.role ? mintableForMe(scopes, org.permissions ?? []) : scopes.filter(s => !s.personOnly);
    return allowed.map(s => ({ name: s.name, label: scopeLabel(s.name) }));
  });

  async function projectsFor(id: string) {
    const cached = projectCache.get(id);
    if (cached) {
      return cached;
    }
    try {
      const res = await projectClient.listProjects({ orgId: id });
      const list = (res.projects ?? []).map(p => ({ id: p.id, name: p.name }));
      projectCache.set(id, list);
      return list;
    } catch {
      return [];
    }
  }

  async function selectOrg(id: string) {
    orgId.value = id;
    scopesPicked.value = [];
    allProjects.value = true;
    projectIds.value = [];
    projects.value = await projectsFor(id);
  }

  async function load() {
    await Promise.all([auth.load(), orgs.ensureLoaded()]);
    if (oneOrg.value) {
      await selectOrg(orgs.items[0]!.id);
    }
  }

  async function submit() {
    if (!canSubmit.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.createAPIKey({
        orgId: orgId.value,
        kind: 'personal',
        name: name.value.trim(),
        scopes: scopesPicked.value,
        allProjects: allProjects.value,
        projectIds: allProjects.value ? [] : projectIds.value,
        expiresAt: timestampFromDate(new Date(Date.now() + expiresInDays.value * 86_400_000)),
      });
      createdKeyState().value = { plaintext: res.plaintext, snippet: mcpSnippet('othra', mcpURL(apiUrl, window.location.origin), res.plaintext) };
      await navigateTo('/account/api-keys');
    } catch (e) {
      error.value = errorMessage(e, 'error.create_apikey');
    } finally {
      busy.value = false;
    }
  }

  return {
    name,
    orgId,
    orgName,
    orgOptions,
    oneOrg,
    expiresInDays,
    expiryOptions,
    allProjects,
    projectIds,
    scopes: scopesPicked,
    projects,
    mintable,
    busy,
    error,
    canSubmit,
    selectOrg,
    load,
    submit,
  };
}
