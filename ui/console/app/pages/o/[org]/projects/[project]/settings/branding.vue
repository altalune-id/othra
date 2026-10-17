<script setup lang="ts">
import BrandingPreview from '~/components/project/branding/BrandingPreview.vue';
import { useBranding } from '~/components/project/branding/useBranding';
import ErrorText from '~/components/ui/ErrorText.vue';
import FieldError from '~/components/ui/FieldError.vue';
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

const branding = useBranding(listed.value.id);
const { displayName, primaryColor, supportEmail, preview, contrast, logoError, busy, saved, error, fieldErrors } = branding;
const theme = ref<'othra-light' | 'othra-dark'>('othra-light');
const fileInput = ref<HTMLInputElement>();
const ids = { name: useId(), nameHelp: useId(), nameErr: useId(), logo: useId(), logoHelp: useId(), logoErr: useId(), color: useId(), colorHex: useId(), colorErr: useId(), contrast: useId(), email: useId(), emailHelp: useId(), emailErr: useId() };
const colorValue = computed(() => preview.value.primary || '#000000');

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

onBeforeUnmount(() => branding.dispose());

await branding.load();

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('branding.title')} · Othra` });
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('branding.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('branding.subtitle') }}
      </p>
    </div>

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <form novalidate class="space-y-2" @submit.prevent="branding.save()">
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
            <label :for="ids.logo" class="btn btn-sm" :class="{ 'btn-disabled': busy }">
              <Icon name="lucide:upload" size="16" />
              {{ t('branding.logo_upload') }}
            </label>
            <input :id="ids.logo" ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" :aria-describedby="`${ids.logoHelp} ${ids.logoErr}`" :disabled="busy" @change="onFile">
            <button v-if="preview.logoUrl" type="button" class="btn btn-ghost btn-sm text-error" :disabled="busy" @click="branding.removeLogo()">
              <Icon name="lucide:trash-2" size="16" />
              {{ t('branding.logo_remove') }}
            </button>
          </div>
          <FieldError :id="ids.logoErr" :message="logoError" />
          <p :id="ids.logoHelp" class="label whitespace-normal">
            {{ t('branding.logo_help') }}
          </p>
        </div>

        <div class="fieldset">
          <label :for="ids.colorHex" class="fieldset-legend">{{ t('branding.primary') }}</label>
          <div class="join">
            <input :id="ids.color" type="color" class="join-item h-10 w-12 cursor-pointer border border-base-300 bg-base-100 p-1" :value="colorValue" :aria-label="t('branding.primary')" :disabled="busy" @input="primaryColor = ($event.target as HTMLInputElement).value">
            <input :id="ids.colorHex" v-model="primaryColor" type="text" class="input join-item w-36 font-mono" placeholder="#4f46e5" maxlength="7" :aria-describedby="`${ids.contrast} ${ids.colorErr}`" :disabled="busy">
          </div>
          <FieldError :id="ids.colorErr" :message="fieldErrors.primary_color" />
          <p v-if="contrast" :id="ids.contrast" class="flex items-center gap-1 text-sm" :class="contrast.low ? 'text-warning' : 'text-base-content/60'">
            <Icon :name="contrast.low ? 'lucide:triangle-alert' : 'lucide:check'" size="14" />
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
          <span v-if="saved" class="text-sm text-success">{{ t('common.saved') }}</span>
        </div>
      </form>

      <BrandingPreview v-model:theme="theme" :model="preview" :labels="previewLabels" />
    </div>
  </div>
</template>
