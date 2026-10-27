<script setup lang="ts">
import type { BrandingPreviewModel } from './useBranding';
import { computed } from 'vue';

export interface BrandingPreviewLabels {
  preview: string;
  note: string;
  heading: string;
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
    heading: 'Sign in to continue to Example app',
    email: 'Email',
    cta: 'Email me a sign-in link',
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
const help = computed(() => props.labels.help.replace('{email}', props.model.supportEmail));
</script>

<template>
  <section class="space-y-2">
    <div class="flex items-center justify-between gap-2">
      <h2 class="text-sm font-semibold">
        {{ labels.preview }}
      </h2>
      <div class="join">
        <button type="button" class="btn join-item btn-xs" :class="{ 'btn-active': theme === 'othra-light' }" :aria-pressed="theme === 'othra-light'" @click="theme = 'othra-light'">
          {{ labels.light }}
        </button>
        <button type="button" class="btn join-item btn-xs" :class="{ 'btn-active': theme === 'othra-dark' }" :aria-pressed="theme === 'othra-dark'" @click="theme = 'othra-dark'">
          {{ labels.dark }}
        </button>
      </div>
    </div>
    <div :data-theme="theme" :style="style" class="rounded-box bg-base-200 p-4" data-testid="branding-preview">
      <div class="pointer-events-none card mx-auto max-w-xs border border-base-300 bg-base-100 text-base-content" aria-hidden="true">
        <div class="card-body gap-3 p-5">
          <div class="flex items-center gap-2">
            <img v-if="model.logoUrl" :src="model.logoUrl" alt="" class="size-8 rounded object-contain">
            <span class="font-semibold">{{ model.name }}</span>
          </div>
          <p class="text-sm text-base-content/70">
            {{ labels.heading }}
          </p>
          <input type="email" class="input w-full" :placeholder="labels.email" tabindex="-1">
          <button type="button" class="btn btn-primary btn-sm" tabindex="-1">
            {{ labels.cta }}
          </button>
          <p v-if="model.termsUrl || model.privacyUrl" class="text-xs text-base-content/60">
            <span v-if="model.termsUrl" class="link">{{ labels.terms }}</span>
            <span v-if="model.termsUrl && model.privacyUrl"> · </span>
            <span v-if="model.privacyUrl" class="link">{{ labels.privacy }}</span>
          </p>
          <p v-if="model.supportEmail" class="text-xs text-base-content/60">
            {{ help }}
          </p>
          <p class="text-center text-xs text-base-content/50">
            {{ labels.secured }}
          </p>
        </div>
      </div>
    </div>
    <p class="text-xs text-base-content/60">
      {{ labels.note }}
    </p>
  </section>
</template>
