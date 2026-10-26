import type { Upstream } from '~~/gen/upstream/v1/upstream_pb';
import type { ShownError } from '~/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { watch } from 'vue';
import { CreateUpstreamRequestSchema, UpdateUpstreamRequestSchema, UpstreamKind } from '~~/gen/upstream/v1/upstream_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useValidation } from '~/composables/useValidation';
import { errorCode, errorMeta } from '~/lib/apperror/message';
import { useProjectDetailStore } from '~/stores/projectDetail';

// googleRedirectUri is the callback a project's Google client must register, under the project's issuer.
export function googleRedirectUri(issuerUrl: string): string {
  return issuerUrl ? `${issuerUrl.replace(/\/+$/, '')}/federation/google/callback` : '';
}

// useSignIn edits how a project's end users sign in: the email link and the Google upstream IdP, never leaving the project with no method.
export function useSignIn(projectId: string) {
  const { $projectClient: projectClient, $upstreamClient: client } = useNuxtApp();
  const detail = useProjectDetailStore();
  const errorMessage = useErrorMessage();
  const createRules = useValidation(CreateUpstreamRequestSchema, { fields: ['client_id', 'client_secret'], codes: { UPS003: 'client_id' } });
  const updateRules = useValidation(UpdateUpstreamRequestSchema, { fields: ['client_id', 'client_secret'] });

  const google = ref<Upstream | null>(null);
  const loadError = ref<ShownError | null>(null);
  const settingUp = ref(false);
  const clientId = ref('');
  const secret = ref('');
  const replacingSecret = ref(false);
  const busy = ref(false);
  const saved = ref(false);
  const error = ref<ShownError | null>(null);
  const toggleError = ref<ShownError | null>(null);
  const emailError = ref<ShownError | null>(null);
  const unlinkedRefusal = ref(false);
  const removing = ref(false);
  const removeError = ref<ShownError | null>(null);

  const project = computed(() => detail.get(projectId));
  const emailLink = computed(() => project.value?.signinEmailLink ?? false);
  const googleOn = computed(() => google.value?.enabled ?? false);
  const emailLockReason = computed<'' | 'google_off' | 'unlinked'>(() => {
    if (!emailLink.value) {
      return '';
    }
    if (!googleOn.value) {
      return 'google_off';
    }
    return unlinkedRefusal.value ? 'unlinked' : '';
  });
  const emailLocked = computed(() => emailLockReason.value !== '');
  const googleLocked = computed(() => googleOn.value && !emailLink.value);
  const removeLocked = computed(() => google.value?.identitiesLinked ?? false);
  const redirectUri = computed(() => google.value?.redirectUri || googleRedirectUri(project.value?.issuerUrl ?? ''));
  const fieldErrors = computed(() => (google.value ? updateRules.errors.value : createRules.errors.value));

  watch([clientId, secret], () => {
    saved.value = false;
  }, { flush: 'sync' });

  function reset(next: Upstream | null) {
    google.value = next;
    clientId.value = next?.clientId ?? '';
    secret.value = '';
    replacingSecret.value = false;
  }

  async function load() {
    loadError.value = null;
    try {
      const [, res] = await Promise.all([detail.ensureLoaded(projectId), client.listUpstreams({ projectId })]);
      reset(res.upstreams.find(u => u.kind === UpstreamKind.GOOGLE) ?? null);
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_signin');
    }
  }

  function startSetup() {
    reset(null);
    settingUp.value = true;
  }

  function cancelSetup() {
    settingUp.value = false;
    createRules.clear();
    error.value = null;
  }

  async function save() {
    if (busy.value) {
      return;
    }
    saved.value = false;
    error.value = null;
    const current = google.value;
    if (current) {
      const req = { projectId, upstreamId: current.id, clientId: clientId.value.trim(), clientSecret: replacingSecret.value ? secret.value : '' };
      if (!updateRules.check(create(UpdateUpstreamRequestSchema, req))) {
        error.value = updateRules.formError.value;
        return;
      }
      await run(() => client.updateUpstream(req), updateRules.fromServer);
      return;
    }
    const req = { projectId, kind: UpstreamKind.GOOGLE, clientId: clientId.value.trim(), clientSecret: secret.value, enabled: true };
    if (!createRules.check(create(CreateUpstreamRequestSchema, req))) {
      error.value = createRules.formError.value;
      return;
    }
    await run(() => client.createUpstream(req), createRules.fromServer);
  }

  async function run(call: () => Promise<{ upstream?: Upstream }>, fromServer: (e: unknown, key: string) => ShownError | null) {
    busy.value = true;
    try {
      const res = await call();
      reset(res.upstream ?? null);
      settingUp.value = false;
      saved.value = true;
      emailError.value = null;
    } catch (e) {
      error.value = fromServer(e, 'error.save_signin');
    } finally {
      busy.value = false;
    }
  }

  async function setEmailLink(on: boolean) {
    const current = project.value;
    if (!current || busy.value) {
      return;
    }
    busy.value = true;
    emailError.value = null;
    try {
      const res = await projectClient.updateProjectSignIn({ id: projectId, emailLink: on });
      detail.replace(res.project ?? { ...current, signinEmailLink: on });
      unlinkedRefusal.value = false;
    } catch (e) {
      unlinkedRefusal.value = errorCode(e) === 'PRJ010' && googleOn.value && errorMeta(e).reason !== 'google_off';
      emailError.value = unlinkedRefusal.value ? null : errorMessage(e, 'error.save_signin');
    } finally {
      busy.value = false;
    }
  }

  async function setGoogleEnabled(on: boolean) {
    const current = google.value;
    if (!current || busy.value) {
      return;
    }
    busy.value = true;
    toggleError.value = null;
    try {
      const res = await client.setUpstreamEnabled({ projectId, upstreamId: current.id, enabled: on });
      google.value = res.upstream ?? { ...current, enabled: on };
      emailError.value = null;
    } catch (e) {
      toggleError.value = errorMessage(e, 'error.save_signin');
    } finally {
      busy.value = false;
    }
  }

  function askRemove() {
    removeError.value = null;
    removing.value = true;
  }

  function cancelRemove() {
    if (!busy.value) {
      removing.value = false;
      removeError.value = null;
    }
  }

  async function confirmRemove() {
    const current = google.value;
    if (!current || !removing.value || busy.value) {
      return;
    }
    busy.value = true;
    removeError.value = null;
    try {
      await client.deleteUpstream({ projectId, upstreamId: current.id });
      reset(null);
      removing.value = false;
      emailError.value = null;
    } catch (e) {
      removeError.value = errorMessage(e, 'error.remove_upstream');
    } finally {
      busy.value = false;
    }
  }

  return {
    google,
    loadError,
    settingUp,
    clientId,
    secret,
    replacingSecret,
    busy,
    saved,
    error,
    toggleError,
    emailError,
    removing,
    removeError,
    emailLink,
    googleOn,
    emailLocked,
    emailLockReason,
    googleLocked,
    removeLocked,
    redirectUri,
    fieldErrors,
    load,
    startSetup,
    cancelSetup,
    save,
    setEmailLink,
    setGoogleEnabled,
    askRemove,
    cancelRemove,
    confirmRemove,
  };
}
