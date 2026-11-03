<script setup lang="ts">
import type { BrandingPreviewModel } from './useBranding';
import { computed } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';

export interface BrandingPreviewLabels {
  preview: string;
  note: string;
  title: string;
  heading: string;
  placeholder: string;
  email: string;
  cta: string;
  terms: string;
  privacy: string;
  help: string;
  secured: string;
  light: string;
  dark: string;
}

const props = withDefaults(defineProps<{
  model: BrandingPreviewModel,
  labels?: BrandingPreviewLabels,
}>(), {
  labels: () => ({
    preview: 'Preview',
    note: 'A close approximation of the sign-in page.',
    title: 'Sign in to {project}',
    heading: 'Continue to Example app',
    placeholder: 'you@example.com',
    email: 'Email',
    cta: 'Continue with email',
    terms: 'Terms',
    privacy: 'Privacy',
    help: 'Need help? Contact {email}.',
    secured: 'Secured by Othra',
    light: 'Light',
    dark: 'Dark',
  }),
});

const theme = defineModel<'othra-light' | 'othra-dark'>('theme', { default: 'othra-light' });

const style = computed(() => (props.model.primary
  ? { '--color-primary': props.model.primary, '--color-primary-content': props.model.primaryContent }
  : {}));
const title = computed(() => props.labels.title.replace('{project}', props.model.name));
const initial = computed(() => [...props.model.name.trim()][0]?.toUpperCase() ?? '');
const help = computed(() => props.labels.help.replace('{email}', props.model.supportEmail));
</script>

<template>
  <section class="flex flex-col gap-2">
    <div class="flex items-center justify-between gap-2">
      <h2 class="text-ui font-semibold">
        {{ labels.preview }}
      </h2>
      <div class="join">
        <AppButton variant="plain" size="xs" class="join-item" :class="{ 'btn-active': theme === 'othra-light' }" :aria-pressed="theme === 'othra-light'" @click="theme = 'othra-light'">
          {{ labels.light }}
        </AppButton>
        <AppButton variant="plain" size="xs" class="join-item" :class="{ 'btn-active': theme === 'othra-dark' }" :aria-pressed="theme === 'othra-dark'" @click="theme = 'othra-dark'">
          {{ labels.dark }}
        </AppButton>
      </div>
    </div>
    <div :data-theme="theme" :style="style" class="rounded-box border border-base-300 bg-base-100 px-5 py-6 text-sm leading-[normal] text-base-content" data-testid="branding-preview">
      <div class="pointer-events-none flex flex-col gap-5" aria-hidden="true">
        <div class="flex flex-col gap-3">
          <img v-if="model.logoUrl" :src="model.logoUrl" alt="" class="size-10 rounded-field border border-base-300 bg-base-100 object-contain p-1">
          <span v-else-if="initial" class="grid size-10 place-items-center rounded-field bg-primary text-lg font-semibold text-primary-content">{{ initial }}</span>
          <p class="text-xl font-semibold text-balance break-words">
            {{ title }}
          </p>
          <p class="-mt-2 text-base-content/70">
            {{ labels.heading }}
          </p>
        </div>
        <div class="flex flex-col gap-3">
          <label class="fieldset gap-1 p-0">
            <span class="label text-ui text-base-content">{{ labels.email }}</span>
            <input type="email" class="input w-full" :placeholder="labels.placeholder" tabindex="-1">
          </label>
          <span class="btn btn-primary w-full">{{ labels.cta }}</span>
        </div>
        <p v-if="model.termsUrl || model.privacyUrl" class="text-ui text-base-content/70">
          <span v-if="model.termsUrl" class="link">{{ labels.terms }}</span>
          <span v-if="model.termsUrl && model.privacyUrl"> · </span>
          <span v-if="model.privacyUrl" class="link">{{ labels.privacy }}</span>
        </p>
        <p v-if="model.supportEmail" class="text-xs text-base-content/70">
          {{ help }}
        </p>
        <p class="text-center text-xs text-base-content/70">
          {{ labels.secured }}
        </p>
      </div>
    </div>
    <p class="text-xs text-base-content/70">
      {{ labels.note }}
    </p>
  </section>
</template>
