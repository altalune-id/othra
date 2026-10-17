import type { ShownError } from '~/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { UpdateProjectBrandingRequestSchema } from '~~/gen/project/v1/project_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useValidation } from '~/composables/useValidation';
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

const logoTypes = ['image/png', 'image/jpeg', 'image/webp'];
const logoMaxBytes = 262_144;

// useBranding edits a project's display name, logo, primary colour and support email, and models the sign-in preview.
export function useBranding(projectId: string) {
  const { $projectClient: client } = useNuxtApp();
  const { t } = useI18n();
  const detail = useProjectDetailStore();
  const errorMessage = useErrorMessage();
  const validation = useValidation(UpdateProjectBrandingRequestSchema, { fields: ['display_name', 'primary_color', 'support_email'] });

  const displayName = ref('');
  const primaryColor = ref('');
  const supportEmail = ref('');
  const logoUrl = ref('');
  const logoError = ref('');
  const busy = ref(false);
  const saved = ref(false);
  const error = ref<ShownError | null>(null);

  const project = computed(() => detail.get(projectId));
  const hex = computed(() => normalizeHex(primaryColor.value));
  const contrast = computed(() => {
    if (!hex.value) {
      return null;
    }
    return { ratio: formatRatio(contrastRatio(hex.value, bestTextColor(hex.value))), low: isLowContrast(hex.value) };
  });
  const preview = computed<BrandingPreviewModel>(() => ({
    name: displayName.value.trim() || project.value?.name || '',
    logoUrl: logoUrl.value,
    primary: hex.value,
    primaryContent: hex.value ? bestTextColor(hex.value) : '',
    termsUrl: project.value?.termsUrl ?? '',
    privacyUrl: project.value?.privacyUrl ?? '',
    supportEmail: supportEmail.value.trim(),
  }));

  function setLogoUrl(next: string) {
    if (logoUrl.value) {
      URL.revokeObjectURL(logoUrl.value);
    }
    logoUrl.value = next;
  }

  async function loadLogo() {
    if (!project.value?.hasLogo) {
      setLogoUrl('');
      return;
    }
    const res = await client.getProjectLogo({ id: projectId });
    setLogoUrl(URL.createObjectURL(new Blob([res.content as BlobPart], { type: res.contentType })));
  }

  function sync() {
    displayName.value = project.value?.displayName ?? '';
    primaryColor.value = project.value?.primaryColor ?? '';
    supportEmail.value = project.value?.supportEmail ?? '';
  }

  async function load() {
    await detail.ensureLoaded(projectId);
    sync();
    await loadLogo();
  }

  async function pickLogo(file: File) {
    logoError.value = '';
    if (!logoTypes.includes(file.type)) {
      logoError.value = t('apperror.PRJ008_type');
      return;
    }
    if (file.size > logoMaxBytes) {
      logoError.value = t('apperror.PRJ008_size', { max: logoMaxBytes / 1024 });
      return;
    }
    busy.value = true;
    try {
      const res = await client.uploadProjectLogo({ id: projectId, content: new Uint8Array(await file.arrayBuffer()) });
      if (res.project) {
        detail.replace(res.project);
      }
      await loadLogo();
    } catch (e) {
      logoError.value = errorMessage(e, 'error.upload_logo').message;
    } finally {
      busy.value = false;
    }
  }

  async function removeLogo() {
    busy.value = true;
    logoError.value = '';
    try {
      const res = await client.deleteProjectLogo({ id: projectId });
      if (res.project) {
        detail.replace(res.project);
      }
      setLogoUrl('');
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
    validation.clear();
  }

  function dispose() {
    setLogoUrl('');
  }

  return {
    displayName,
    primaryColor,
    supportEmail,
    preview,
    contrast,
    logoError,
    busy,
    saved,
    error,
    fieldErrors: validation.errors,
    load,
    pickLogo,
    removeLogo,
    save,
    resetDefaults,
    dispose,
  };
}
