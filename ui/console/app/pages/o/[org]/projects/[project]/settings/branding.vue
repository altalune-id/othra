<script setup lang="ts">
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import BrandingPreview from '~/components/project/branding/BrandingPreview.vue';
import { useBranding } from '~/components/project/branding/useBranding';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const canEdit = useCanEditProject();
const branding = useBranding(listed.value.id);
const { displayName, primaryColor, supportEmail, preview, contrast, logoError, uploading, logoSaved, storageOff, busy, saved, error, fieldErrors } = branding;
const theme = ref<'othra-light' | 'othra-dark'>('othra-light');
const fileInput = ref<HTMLInputElement>();
const ids = { name: useId(), nameHelp: useId(), nameErr: useId(), logo: useId(), logoHelp: useId(), logoErr: useId(), color: useId(), colorHex: useId(), colorErr: useId(), colorDefault: useId(), contrast: useId(), email: useId(), emailHelp: useId(), emailErr: useId() };
const colorValue = computed(() => preview.value.primary || '#4f46e5');

const previewLabels = computed(() => ({
  preview: t('branding.preview'),
  note: t('branding.preview_note'),
  heading: t('branding.preview_heading', { client: t('branding.preview_client') }),
  email: t('field.email'),
  cta: t('branding.preview_cta'),
  terms: t('branding.preview_terms'),
  privacy: t('branding.preview_privacy'),
  help: t('branding.preview_help', { email: '{email}' }),
  secured: t('branding.preview_secured'),
  light: t('theme.light'),
  dark: t('theme.dark'),
}));

async function onFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) {
    await branding.pickLogo(file);
  }
  input.value = '';
}

await branding.load();

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('branding.title')} · Othra` });
</script>

<template>
  <div class="max-w-5xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('branding.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('branding.subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" />

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <form v-if="canEdit" novalidate class="card self-start border border-base-300 bg-base-100" @submit.prevent="branding.save()">
        <div class="card-body gap-2">
          <div class="fieldset">
            <label :for="ids.name" class="fieldset-legend">{{ t('branding.display_name') }}</label>
            <input :id="ids.name" v-model="displayName" type="text" class="input w-full" maxlength="60" :placeholder="listed?.name" :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`" :disabled="busy">
            <FieldError :id="ids.nameErr" :message="fieldErrors.display_name" />
            <p :id="ids.nameHelp" class="label whitespace-normal">
              {{ t('branding.display_name_help') }}
            </p>
          </div>

          <div class="fieldset">
            <span class="fieldset-legend">{{ t('branding.logo') }}</span>
            <div class="flex flex-wrap items-center gap-3">
              <div class="flex size-16 items-center justify-center rounded-box border border-base-300 bg-base-200">
                <img v-if="preview.logoUrl" :src="preview.logoUrl" alt="" class="size-14 object-contain">
                <Icon v-else name="lucide:image" size="20" class="text-base-content/40" />
              </div>
              <input :id="ids.logo" ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="peer sr-only" :aria-describedby="storageOff ? ids.logoErr : `${ids.logoHelp} ${ids.logoErr}`" :disabled="busy || uploading || storageOff" @change="onFile">
              <label :for="ids.logo" class="btn btn-sm peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-base-content" :class="{ 'btn-disabled': busy || uploading || storageOff }" :aria-disabled="busy || uploading || storageOff">
                <span v-if="uploading" class="loading loading-spinner loading-xs" />
                <Icon v-else name="lucide:upload" size="16" />
                {{ uploading ? t('branding.logo_uploading') : t('branding.logo_upload') }}
              </label>
              <button v-if="preview.logoUrl" type="button" class="btn btn-ghost btn-sm" :disabled="busy || uploading" @click="branding.removeLogo()">
                <Icon name="lucide:trash-2" size="16" class="text-error" />
                {{ t('branding.logo_remove') }}
              </button>
            </div>
            <div v-if="storageOff" role="note" class="alert alert-soft">
              <Icon name="lucide:info" size="16" class="text-info" />
              <span class="text-sm text-base-content">{{ t('branding.storage_off') }}</span>
            </div>
            <FieldError :id="ids.logoErr" :message="logoError" />
            <SavedNote v-if="logoSaved" :label="t('branding.logo_updated')" />
            <p v-if="!storageOff" :id="ids.logoHelp" class="label whitespace-normal">
              {{ t('branding.logo_help') }} {{ t('branding.logo_instant') }}
            </p>
          </div>

          <div class="fieldset">
            <label :for="ids.colorHex" class="fieldset-legend">{{ t('branding.primary') }}</label>
            <div class="flex flex-wrap items-center gap-2">
              <div class="join">
                <input :id="ids.color" type="color" class="join-item h-10 w-12 cursor-pointer border border-base-300 bg-base-100 p-1" :value="colorValue" :aria-label="t('branding.pick_primary')" :aria-describedby="preview.primary ? undefined : ids.colorDefault" :disabled="busy" @input="primaryColor = ($event.target as HTMLInputElement).value">
                <input :id="ids.colorHex" v-model="primaryColor" type="text" class="input join-item w-36 font-mono" placeholder="#4f46e5" maxlength="7" :aria-describedby="`${ids.contrast} ${ids.colorErr}`" :disabled="busy">
              </div>
              <span v-if="!preview.primary" :id="ids.colorDefault" class="text-sm text-base-content/70">{{ t('branding.default_colour') }}</span>
            </div>
            <FieldError :id="ids.colorErr" :message="fieldErrors.primary_color" />
            <p v-if="contrast" :id="ids.contrast" class="flex items-center gap-1 text-sm text-base-content/70">
              <Icon :name="contrast.low ? 'lucide:triangle-alert' : 'lucide:check'" size="14" :class="contrast.low ? 'text-warning' : 'text-success'" />
              {{ t('branding.contrast', { ratio: contrast.ratio }) }}
              <span v-if="contrast.low">· {{ t('branding.contrast_low') }}</span>
            </p>
          </div>

          <div class="fieldset">
            <label :for="ids.email" class="fieldset-legend">{{ t('branding.support_email') }}</label>
            <input :id="ids.email" v-model="supportEmail" type="email" class="input w-full" placeholder="help@example.com" :aria-describedby="`${ids.emailHelp} ${ids.emailErr}`" :disabled="busy">
            <FieldError :id="ids.emailErr" :message="fieldErrors.support_email" />
            <p :id="ids.emailHelp" class="label whitespace-normal">
              {{ t('branding.support_email_help') }}
            </p>
          </div>

          <ErrorText :error="error" />
          <div class="flex flex-wrap items-center gap-2 pt-2">
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
              {{ busy ? t('common.saving') : t('common.save') }}
            </button>
            <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="branding.resetDefaults()">
              {{ t('branding.reset') }}
            </button>
            <SavedNote v-if="saved" :label="t('common.saved')" />
          </div>
        </div>
      </form>
      <dl v-else class="card self-start border border-base-300 bg-base-100" data-testid="branding-readonly">
        <div class="card-body gap-4">
          <div>
            <dt class="fieldset-legend">
              {{ t('branding.display_name') }}
            </dt>
            <dd>{{ preview.name }}</dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('branding.logo') }}
            </dt>
            <dd>
              <img v-if="preview.logoUrl" :src="preview.logoUrl" alt="" class="size-14 rounded-box border border-base-300 object-contain">
              <span v-else class="text-base-content/50">—</span>
            </dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('branding.primary') }}
            </dt>
            <dd class="flex items-center gap-2 font-mono text-sm">
              <template v-if="preview.primary">
                <span class="inline-block size-4 rounded border border-base-300" :style="{ backgroundColor: preview.primary }" />
                {{ preview.primary }}
              </template>
              <span v-else class="text-base-content/50">—</span>
            </dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('branding.support_email') }}
            </dt>
            <dd>
              <template v-if="preview.supportEmail">
                {{ preview.supportEmail }}
              </template>
              <span v-else class="text-base-content/50">—</span>
            </dd>
          </div>
        </div>
      </dl>

      <BrandingPreview v-model:theme="theme" :model="preview" :labels="previewLabels" />
    </div>
  </div>
</template>
