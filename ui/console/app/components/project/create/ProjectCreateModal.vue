<script setup lang="ts">
import AppModal from '~/components/ui/AppModal.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import FieldError from '~/components/ui/FieldError.vue';
import IssuerHandleField from '../issuer/IssuerHandleField.vue';
import { useProjectCreate } from './useProjectCreate';

const { orgId } = defineProps<{ orgId: string }>();

const { t } = useI18n();
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
  locked: t('issuer.locked'),
  error: t('issuer.check_failed'),
}));
</script>

<template>
  <AppModal :open="open" :title="t('project.new')" @close="hide()">
    <form novalidate class="space-y-3" @submit.prevent="submit()">
      <label class="block">
        <span class="mb-1 block text-xs text-base-content/60">{{ t('common.name') }}</span>
        <input
          v-model="name" type="text" class="input input-sm input-bordered w-full" :class="{ 'input-error': fieldErrors.name }"
          placeholder="Support Bot" :disabled="busy" :aria-invalid="!!fieldErrors.name" aria-describedby="project-name-error"
        >
        <FieldError id="project-name-error" :message="fieldErrors.name" />
      </label>
      <label class="block">
        <span class="mb-1 block text-xs text-base-content/60">{{ t('project.console_slug') }}</span>
        <input
          :value="slug" type="text" class="input input-sm input-bordered w-full" :class="{ 'input-error': fieldErrors.slug }"
          placeholder="support-bot (generated when empty)" :disabled="busy" :aria-invalid="!!fieldErrors.slug" aria-describedby="project-slug-error"
          @input="create.setSlug(($event.target as HTMLInputElement).value)"
        >
        <FieldError id="project-slug-error" :message="fieldErrors.slug" />
      </label>
      <IssuerHandleField
        :model-value="handleValue"
        :status="handleStatus"
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
