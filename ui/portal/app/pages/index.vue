<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
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
  <div class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('nav.profile') }}
      </h1>
      <p class="text-sm text-base-content/70">
        {{ t('profile.subtitle', { project: config.project }) }}
      </p>
    </div>
    <LoadError :error="account.loadError" :busy="account.loading" :retry-label="t('common.retry')" @retry="account.load()" />

    <form novalidate class="card border border-base-300 bg-base-100" @submit.prevent="form.save()">
      <div class="card-body gap-2">
        <div class="fieldset">
          <label :for="ids.name" class="fieldset-legend">{{ t('profile.name') }}</label>
          <input
            :id="ids.name"
            v-model="name"
            class="input w-full"
            :class="{ 'input-error': fieldErrors.name }"
            autocomplete="name"
            :disabled="busy"
            :aria-invalid="!!fieldErrors.name"
            :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`"
          >
          <FieldError :id="ids.nameErr" :message="fieldErrors.name" />
          <p :id="ids.nameHelp" class="label whitespace-normal">
            {{ t('profile.name_help') }}
          </p>
        </div>
        <ErrorText :error="error" />
        <div class="flex items-center gap-3 pt-1">
          <AppButton type="submit" size="sm" :disabled="busy">
            {{ busy ? t('common.saving') : t('common.save') }}
          </AppButton>
          <SavedNote v-if="saved" :label="t('common.saved')" />
        </div>
      </div>
    </form>

    <section class="card border border-base-300 bg-base-100" :aria-labelledby="ids.email">
      <div class="card-body gap-1">
        <h2 :id="ids.email" class="fieldset-legend">
          {{ t('profile.email') }}
        </h2>
        <p class="font-medium break-all">
          {{ account.profile?.email }}
        </p>
        <p class="text-sm text-base-content/70">
          {{ t('profile.email_help') }}
        </p>
      </div>
    </section>
  </div>
</template>
