import type { ShownError } from '~/lib/apperror/message';
import { UpdateProjectIssuerRequestSchema } from '~~/gen/project/v1/project_pb';
import { useValidation } from '~/composables/useValidation';
import { useProjectDetailStore } from '~/stores/projectDetail';

export type HandleStatus = 'idle' | 'checking' | 'available' | 'taken' | 'reserved' | 'invalid' | 'locked' | 'error';

const handleMax = 40;
const handlePattern = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;
const serverStatus: Record<string, HandleStatus> = { taken: 'taken', reserved: 'reserved', locked: 'locked', invalid: 'invalid' };

// isHandleFormat applies the issuer handle rules the server enforces: 3–40 lowercase letters, digits and inner hyphens.
export function isHandleFormat(handle: string): boolean {
  return handle.length >= 3 && handle.length <= handleMax && handlePattern.test(handle);
}

// defaultHandle is the create form's prefill, <org slug>-<project slug>, cut to 40 characters.
export function defaultHandle(orgSlug: string, projectSlug: string): string {
  if (!projectSlug) {
    return '';
  }
  return `${orgSlug}-${projectSlug}`.slice(0, handleMax).replace(/-+$/, '');
}

export interface IssuerHandleOptions {
  orgId: string;
  projectId?: string;
  initial: string;
  locked?: boolean;
  delayMs?: number;
}

export function useIssuerHandle(options: IssuerHandleOptions) {
  const { $projectClient: client } = useNuxtApp();
  const detail = useProjectDetailStore();
  const validation = useValidation(UpdateProjectIssuerRequestSchema, {
    fields: ['issuer_handle'],
    codes: { PRJ005: 'issuer_handle', PRJ006: 'issuer_handle', PRJ007: 'issuer_handle' },
  });

  const saved = ref(options.initial);
  const value = ref(options.initial);
  const status = ref<HandleStatus>(options.locked ? 'locked' : 'idle');
  const touched = ref(false);
  const confirming = ref(false);
  const busy = ref(false);
  const done = ref(false);
  const error = ref<ShownError | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const canSave = computed(() => status.value === 'available' && !busy.value);
  const fieldError = computed(() => validation.errors.value.issuer_handle ?? '');

  async function check(handle: string) {
    const mine = ++seq;
    try {
      const res = await client.checkIssuerHandle({ orgId: options.orgId, projectId: options.projectId ?? '', issuerHandle: handle });
      if (mine === seq) {
        status.value = res.available ? 'available' : serverStatus[res.reason] ?? 'invalid';
      }
    } catch {
      if (mine === seq) {
        status.value = 'error';
      }
    }
  }

  function update(handle: string) {
    value.value = handle;
    done.value = false;
    validation.clear();
    clearTimeout(timer);
    seq++;
    if (options.locked) {
      status.value = 'locked';
      return;
    }
    if (handle === saved.value) {
      status.value = 'idle';
      return;
    }
    if (!isHandleFormat(handle)) {
      status.value = 'invalid';
      return;
    }
    status.value = 'checking';
    timer = setTimeout(check, options.delayMs ?? 400, handle);
  }

  function input(handle: string) {
    touched.value = true;
    update(handle);
  }

  function follow(orgSlug: string, projectSlug: string) {
    if (!touched.value) {
      update(defaultHandle(orgSlug, projectSlug));
    }
  }

  function reset(handle: string) {
    clearTimeout(timer);
    seq++;
    saved.value = handle;
    value.value = handle;
    touched.value = false;
    status.value = options.locked ? 'locked' : 'idle';
  }

  function ask() {
    if (canSave.value) {
      confirming.value = true;
    }
  }

  function cancel() {
    confirming.value = false;
  }

  async function save() {
    if (!canSave.value || !options.projectId) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.updateProjectIssuer({ id: options.projectId, issuerHandle: value.value });
      if (res.project) {
        detail.replace(res.project);
        reset(res.project.issuerHandle);
      }
      done.value = true;
    } catch (e) {
      error.value = validation.fromServer(e, 'error.update_issuer');
    } finally {
      busy.value = false;
      confirming.value = false;
    }
  }

  return { value, status, canSave, confirming, busy, saved: done, error, fieldError, input, follow, reset, ask, cancel, save };
}
