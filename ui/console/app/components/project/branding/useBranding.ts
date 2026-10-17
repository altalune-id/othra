import type { ShownError } from '~/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { watch } from 'vue';
import { UpdateProjectBrandingRequestSchema } from '~~/gen/project/v1/project_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useValidation } from '~/composables/useValidation';
import { errorCode } from '~/lib/apperror/message';
import { useAuthStore } from '~/stores/auth';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { bestTextColor, contrastRatio, formatRatio, isLowContrast, normalizeHex } from './contrast';

export interface BrandingPreviewModel {
  name: string;
  logoUrl: string;
  primary: string;
  primaryContent: string;
  termsUrl: string;
  privacyUrl: string;
  supportEmail: string;
}

// NOTE: mirrors project.LogoPurpose; the server is authoritative, this pre-check is UX only.
const logoTypes = ['image/png', 'image/jpeg', 'image/webp'];
const logoMaxBytes = 262_144;
const browserSetHeaders = new Set(['content-length', 'host']);

// putHeaders drops the signed headers a browser sets itself; it sends the real Content-Length of the file body.
export function putHeaders(signed: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(signed).filter(([name]) => !browserSetHeaders.has(name.toLowerCase())));
}

// useBranding edits a project's display name, logo, primary colour and support email, and models the sign-in preview.
export function useBranding(projectId: string) {
  const { $projectClient: client } = useNuxtApp();
  const { t } = useI18n();
  const detail = useProjectDetailStore();
  const auth = useAuthStore();
  const errorMessage = useErrorMessage();
  const validation = useValidation(UpdateProjectBrandingRequestSchema, { fields: ['display_name', 'primary_color', 'support_email'] });

  const displayName = ref('');
  const primaryColor = ref('');
  const supportEmail = ref('');
  const logoError = ref('');
  const uploading = ref(false);
  const logoSaved = ref(false);
  const storageRefused = ref(false);
  const busy = ref(false);
  const saved = ref(false);
  const error = ref<ShownError | null>(null);

  const project = computed(() => detail.get(projectId));
  const storageOff = computed(() => storageRefused.value || auth.me?.fileUploads === false);
  const hex = computed(() => normalizeHex(primaryColor.value));
  const contrast = computed(() => {
    if (!hex.value) {
      return null;
    }
    return { ratio: formatRatio(contrastRatio(hex.value, bestTextColor(hex.value))), low: isLowContrast(hex.value) };
  });
  const preview = computed<BrandingPreviewModel>(() => ({
    name: displayName.value.trim() || project.value?.name || '',
    logoUrl: project.value?.logoUrl ?? '',
    primary: hex.value,
    primaryContent: hex.value ? bestTextColor(hex.value) : '',
    termsUrl: project.value?.termsUrl ?? '',
    privacyUrl: project.value?.privacyUrl ?? '',
    supportEmail: supportEmail.value.trim(),
  }));

  watch([displayName, primaryColor, supportEmail], () => {
    saved.value = false;
  }, { flush: 'sync' });

  function sync() {
    displayName.value = project.value?.displayName ?? '';
    primaryColor.value = project.value?.primaryColor ?? '';
    supportEmail.value = project.value?.supportEmail ?? '';
  }

  async function load() {
    await Promise.all([detail.ensureLoaded(projectId), auth.load()]);
    sync();
  }

  async function pickLogo(file: File) {
    logoError.value = '';
    logoSaved.value = false;
    if (storageOff.value || uploading.value) {
      return;
    }
    if (!logoTypes.includes(file.type)) {
      logoError.value = t('apperror.PRJ008_type');
      return;
    }
    if (file.size > logoMaxBytes) {
      logoError.value = t('apperror.PRJ008_size', { max: logoMaxBytes / 1024 });
      return;
    }
    uploading.value = true;
    try {
      const upload = await client.createProjectLogoUpload({ id: projectId, contentType: file.type, sizeBytes: file.size });
      const res = await fetch(upload.url, { method: upload.method, headers: putHeaders(upload.headers), body: file }).catch(() => null);
      if (!res?.ok) {
        logoError.value = t('branding.logo_upload_failed');
        return;
      }
      const done = await client.completeProjectLogoUpload({ id: projectId, uploadId: upload.uploadId });
      if (done.project) {
        detail.replace(done.project);
        logoSaved.value = true;
      }
    } catch (e) {
      if (errorCode(e) === 'STO003') {
        storageRefused.value = true;
        return;
      }
      logoError.value = errorMessage(e, 'error.upload_logo').message;
    } finally {
      uploading.value = false;
    }
  }

  async function removeLogo() {
    busy.value = true;
    logoError.value = '';
    logoSaved.value = false;
    try {
      const res = await client.deleteProjectLogo({ id: projectId });
      if (res.project) {
        detail.replace(res.project);
      }
    } catch (e) {
      logoError.value = errorMessage(e, 'error.remove_logo').message;
    } finally {
      busy.value = false;
    }
  }

  async function save() {
    const req = {
      id: projectId,
      displayName: displayName.value.trim(),
      primaryColor: hex.value || primaryColor.value.trim(),
      supportEmail: supportEmail.value.trim(),
    };
    if (!validation.check(create(UpdateProjectBrandingRequestSchema, req))) {
      error.value = validation.formError.value;
      return;
    }
    busy.value = true;
    error.value = null;
    saved.value = false;
    try {
      const res = await client.updateProjectBranding(req);
      if (res.project) {
        detail.replace(res.project);
        sync();
      }
      saved.value = true;
    } catch (e) {
      error.value = validation.fromServer(e, 'error.update_branding');
    } finally {
      busy.value = false;
    }
  }

  function resetDefaults() {
    displayName.value = '';
    primaryColor.value = '';
    supportEmail.value = '';
    saved.value = false;
    validation.clear();
  }

  return {
    displayName,
    primaryColor,
    supportEmail,
    preview,
    contrast,
    logoError,
    uploading,
    logoSaved,
    storageOff,
    busy,
    saved,
    error,
    fieldErrors: validation.errors,
    load,
    pickLogo,
    removeLogo,
    save,
    resetDefaults,
  };
}
