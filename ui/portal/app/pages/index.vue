<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import Section from '#base/components/ui/Section.vue';
import { useProfileForm } from '~/components/portal/useProfileForm';
import { useAccountStore } from '~/stores/account';

const { t } = useI18n();
const config = useRuntimeConfig().public;
const account = useAccountStore();
const form = useProfileForm();
const { name, busy, saved, error, fieldErrors } = form;
const ids = { name: useId(), nameHelp: useId(), nameErr: useId(), email: useId() };

useHead({ title: () => t('nav.profile') });
</script>

<template>
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('nav.profile')" :description="t('profile.subtitle', { project: config.project })" />
    <LoadError :error="account.loadError" :busy="account.loading" :retry-label="t('common.retry')" @retry="account.load()" />

    <form novalidate class="flex flex-col gap-4 rounded-box border border-base-300 bg-base-100 p-4" @submit.prevent="form.save()">
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('profile.name') }}</span>
          <input
            :id="ids.name"
            v-model="name"
            class="input input-sm w-full text-ui"
            :class="{ 'input-error': fieldErrors.name }"
            autocomplete="name"
            :disabled="busy"
            :aria-invalid="!!fieldErrors.name"
            :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`"
          >
        </label>
        <FieldError :id="ids.nameErr" :message="fieldErrors.name" />
        <p :id="ids.nameHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('profile.name_help') }}
        </p>
      </div>
      <ErrorText :error="error" />
      <div class="flex items-center gap-3">
        <AppButton type="submit" size="sm" :disabled="busy">
          {{ busy ? t('common.saving') : t('common.save') }}
        </AppButton>
        <SavedNote v-if="saved" :label="t('common.saved')" />
      </div>
    </form>

    <Section :title="t('profile.email')" :description="t('profile.email_help')">
      <p class="text-ui font-medium break-all">
        {{ account.profile?.email }}
      </p>
    </Section>
  </div>
</template>
