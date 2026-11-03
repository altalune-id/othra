<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import { useLanding } from '~/components/layout/useLanding';
import OrgFields from '~/components/org/create/OrgFields.vue';
import { useOnboarding } from '~/components/org/onboarding/useOnboarding';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';

const { t } = useI18n();
const orgs = useOrgsStore();
const auth = useAuthStore();
const { resolve } = useLanding();
const { name, slug, busy, error, fieldErrors, submit } = useOnboarding();

await Promise.all([orgs.ensureLoaded(), auth.load()]);

if (!orgs.error && (orgs.items.length > 0 || auth.me?.orgCreation !== true)) {
  await navigateTo(await resolve(), { replace: true });
}

useHead({ title: () => `${t('onboarding.title')} · Othra` });
</script>

<template>
  <div class="mx-auto w-full max-w-lg px-4 py-8 sm:px-6 sm:py-16">
    <ErrorText v-if="orgs.error" :error="orgs.error" />
    <form v-else novalidate class="flex flex-col gap-4 rounded-box border border-base-300 bg-base-100 p-6" @submit.prevent="submit()">
      <div class="flex flex-col gap-1">
        <Icon name="lucide:building-2" size="24" class="shrink-0" aria-hidden="true" />
        <h1 class="text-xl font-semibold">
          {{ t('onboarding.title') }}
        </h1>
        <p class="text-ui text-base-content/70">
          {{ t('onboarding.body') }}
        </p>
      </div>
      <OrgFields v-model:name="name" v-model:slug="slug" :field-errors="fieldErrors" :busy="busy" />
      <p class="text-xs text-base-content/70">
        {{ t('onboarding.project_note') }}
      </p>
      <ErrorText :error="error" />
      <AppButton type="submit" size="sm" class="btn-block" :busy="busy" :disabled="!name.trim()">
        {{ busy ? t('common.creating') : t('onboarding.submit') }}
      </AppButton>
    </form>
  </div>
</template>
