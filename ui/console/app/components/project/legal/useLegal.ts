import type { ShownError } from '~/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { UpdateProjectTermsRequestSchema } from '~~/gen/project/v1/project_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useValidation } from '~/composables/useValidation';
import { useProjectDetailStore } from '~/stores/projectDetail';

// useLegal edits a project's terms and privacy links and publishes new terms versions.
export function useLegal(projectId: string) {
  const { $projectClient: client } = useNuxtApp();
  const detail = useProjectDetailStore();
  const errorMessage = useErrorMessage();
  const validation = useValidation(UpdateProjectTermsRequestSchema, { fields: ['terms_url', 'privacy_url'] });

  const termsUrl = ref('');
  const privacyUrl = ref('');
  const busy = ref(false);
  const saved = ref(false);
  const clearing = ref(false);
  const bumping = ref(false);
  const bumpedTo = ref(0);
  const error = ref<ShownError | null>(null);

  const project = computed(() => detail.get(projectId));
  const savedTerms = computed(() => project.value?.termsUrl ?? '');
  const hasTerms = computed(() => savedTerms.value !== '');
  const version = computed(() => project.value?.termsVersion ?? 0);
  const nextVersion = computed(() => version.value + 1);
  const publishedAt = computed(() => (hasTerms.value && project.value?.termsPublishedAt ? timestampDate(project.value.termsPublishedAt) : undefined));
  const urlChangedHint = computed(() => {
    const next = termsUrl.value.trim();
    return version.value >= 1 && next !== '' && next !== savedTerms.value;
  });

  function sync() {
    termsUrl.value = project.value?.termsUrl ?? '';
    privacyUrl.value = project.value?.privacyUrl ?? '';
  }

  async function load() {
    await detail.ensureLoaded(projectId);
    sync();
  }

  async function send() {
    const req = { id: projectId, termsUrl: termsUrl.value.trim(), privacyUrl: privacyUrl.value.trim() };
    if (!validation.check(create(UpdateProjectTermsRequestSchema, req))) {
      error.value = validation.formError.value;
      return;
    }
    busy.value = true;
    error.value = null;
    saved.value = false;
    try {
      const res = await client.updateProjectTerms(req);
      if (res.project) {
        detail.replace(res.project);
        sync();
      }
      saved.value = true;
    } catch (e) {
      error.value = validation.fromServer(e, 'error.update_terms');
    } finally {
      busy.value = false;
    }
  }

  async function save() {
    if (hasTerms.value && termsUrl.value.trim() === '') {
      clearing.value = true;
      return;
    }
    await send();
  }

  async function confirmClear() {
    clearing.value = false;
    await send();
  }

  function cancelClear() {
    clearing.value = false;
  }

  function askBump() {
    if (hasTerms.value) {
      bumpedTo.value = 0;
      bumping.value = true;
    }
  }

  function cancelBump() {
    bumping.value = false;
  }

  async function confirmBump() {
    const current = project.value;
    if (!current) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.bumpTermsVersion({ id: projectId });
      detail.replace({ ...current, termsVersion: res.version, termsPublishedAt: res.publishedAt });
      bumpedTo.value = res.version;
    } catch (e) {
      error.value = errorMessage(e, 'error.bump_terms');
    } finally {
      busy.value = false;
      bumping.value = false;
    }
  }

  return {
    termsUrl,
    privacyUrl,
    hasTerms,
    version,
    nextVersion,
    publishedAt,
    urlChangedHint,
    busy,
    saved,
    clearing,
    bumping,
    bumpedTo,
    error,
    fieldErrors: validation.errors,
    load,
    save,
    confirmClear,
    cancelClear,
    askBump,
    cancelBump,
    confirmBump,
  };
}
