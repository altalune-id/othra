<script setup lang="ts">
import AppModal from '~/components/ui/AppModal.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import FieldError from '~/components/ui/FieldError.vue';
import IssuerHandleField from '../issuer/IssuerHandleField.vue';
import { useProjectCreate } from './useProjectCreate';

const { orgId } = defineProps<{ orgId: string }>();

const { t } = useI18n();
const ids = { name: useId(), nameErr: useId(), slug: useId(), slugHelp: useId(), slugErr: useId() };
const create = useProjectCreate(orgId);
const { open, name, slug, handle, busy, canSubmit, error, fieldErrors, hide, submit } = create;
const { value: handleValue, status: handleStatus } = handle;
const handleLabels = computed(() => ({
  handle: t('issuer.handle'),
  help: t('issuer.handle_help'),
  checking: t('issuer.checking'),
  available: t('issuer.available'),
  taken: t('issuer.taken'),
  reserved: t('issuer.reserved'),
  invalid: t('issuer.invalid'),
  invalidFormat: t('apperror.PRJ006_format'),
  invalidLength: t('apperror.PRJ006_length', { min: 3, max: 40 }),
  locked: t('issuer.locked'),
  error: t('issuer.check_failed'),
}));
</script>

<template>
  <AppModal :open="open" :title="t('project.new')" @close="hide()">
    <form novalidate class="space-y-1" @submit.prevent="submit()">
      <div class="fieldset">
        <label :for="ids.name" class="fieldset-legend">{{ t('common.name') }}</label>
        <input
          :id="ids.name" v-model="name" type="text" class="input w-full" :class="{ 'input-error': fieldErrors.name }"
          :placeholder="t('project.name_placeholder')" :disabled="busy" :aria-invalid="!!fieldErrors.name" :aria-describedby="ids.nameErr"
        >
        <FieldError :id="ids.nameErr" :message="fieldErrors.name" />
      </div>
      <div class="fieldset">
        <label :for="ids.slug" class="fieldset-legend">{{ t('project.console_slug') }}</label>
        <input
          :id="ids.slug" :value="slug" type="text" class="input w-full font-mono" :class="{ 'input-error': fieldErrors.slug }"
          :placeholder="t('project.slug_placeholder')" :disabled="busy" :aria-invalid="!!fieldErrors.slug" :aria-describedby="`${ids.slugHelp} ${ids.slugErr}`"
          @input="create.setSlug(($event.target as HTMLInputElement).value)"
        >
        <FieldError :id="ids.slugErr" :message="fieldErrors.slug" />
        <p :id="ids.slugHelp" class="label whitespace-normal">
          {{ t('project.slug_random_help') }}
        </p>
      </div>
      <IssuerHandleField
        :model-value="handleValue"
        :status="handleStatus"
        prefix="/p/"
        :disabled="busy"
        :field-error="fieldErrors.issuer_handle"
        :labels="handleLabels"
        @update:model-value="handle.input"
      />
      <ErrorText :error="error" />
    </form>
    <template #footer>
      <button class="btn btn-ghost btn-sm" type="button" :disabled="busy" @click="hide()">
        {{ t('common.cancel') }}
      </button>
      <button class="btn btn-primary btn-sm" type="button" :disabled="!canSubmit" @click="submit()">
        {{ busy ? t('common.creating') : t('common.create') }}
      </button>
    </template>
  </AppModal>
</template>
