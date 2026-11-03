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
  newHere: string;
  acceptBoth: string;
  acceptOne: string;
  language: string;
  languageName: string;
  light: string;
  dark: string;
}

const props = withDefaults(defineProps<{
  model: BrandingPreviewModel,
  issuerHost?: string,
  labels?: BrandingPreviewLabels,
}>(), {
  issuerHost: '',
  labels: () => ({
    preview: 'Preview',
    note: 'A close approximation of the sign-in page.',
    title: 'Sign in to {project}',
    heading: 'Continue to Example app',
    placeholder: 'you@example.com',
    email: 'Email',
    cta: 'Continue with email',
    terms: 'Terms of Service',
    privacy: 'Privacy Policy',
    newHere: 'New here? Signing in creates your {project} account.',
    acceptBoth: 'By continuing you accept {project}\'s {terms} and {privacy}.',
    acceptOne: 'By continuing you accept {project}\'s {link}.',
    language: 'Language',
    languageName: 'English',
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
const newHere = computed(() => props.labels.newHere.replace('{project}', props.model.name));

interface AcceptPart { text: string; link: boolean; }

// acceptParts splits the hosted accept sentence around its link placeholders, as the hosted page's splitAround does.
function acceptParts(template: string, links: Record<string, string>): AcceptPart[] {
  return template.replace('{project}', props.model.name).split(/(\{\w+\})/).filter(Boolean).map((part) => {
    const key = part.match(/^\{(\w+)\}$/)?.[1];
    return key && key in links ? { text: links[key]!, link: true } : { text: part, link: false };
  });
}

const accept = computed<AcceptPart[]>(() => {
  const { termsUrl, privacyUrl } = props.model;
  if (termsUrl && privacyUrl) {
    return acceptParts(props.labels.acceptBoth, { terms: props.labels.terms, privacy: props.labels.privacy });
  }
  if (termsUrl || privacyUrl) {
    return acceptParts(props.labels.acceptOne, { link: termsUrl ? props.labels.terms : props.labels.privacy });
  }
  return [];
});
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
    <div :data-theme="theme" :style="style" class="flex flex-col gap-8 rounded-box border border-base-300 bg-base-100 px-5 py-6 text-sm leading-[normal] text-base-content" data-testid="branding-preview">
      <div class="pointer-events-none flex flex-col gap-5" aria-hidden="true">
        <div class="flex flex-col gap-3">
          <img v-if="model.logoUrl" :src="model.logoUrl" alt="" class="size-10 rounded-field border border-base-300 bg-base-100 object-contain p-1">
          <span v-else-if="initial" class="grid size-10 place-items-center rounded-field bg-primary text-lg font-semibold text-primary-content">{{ initial }}</span>
          <p class="text-2xl font-semibold text-balance break-words">
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
        <p class="text-ui text-base-content/70" data-testid="preview-accept">
          {{ newHere }}<template v-if="accept.length">
            {{ ' ' }}<template v-for="(part, i) in accept" :key="i">
              <span v-if="part.link" class="link">{{ part.text }}</span><template v-else>
                {{ part.text }}
              </template>
            </template>
          </template>
        </p>
      </div>
      <div class="pointer-events-none flex flex-col gap-2 text-xs text-base-content/70" aria-hidden="true" data-testid="preview-footer">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <label>
            <span class="sr-only">{{ labels.language }}</span>
            <select class="select select-xs w-28" tabindex="-1">
              <option>{{ labels.languageName }}</option>
            </select>
          </label>
          <span v-if="issuerHost" class="ms-auto font-mono break-all">{{ issuerHost }}</span>
        </div>
      </div>
    </div>
    <p class="text-xs text-base-content/70">
      {{ labels.note }}
    </p>
  </section>
</template>
