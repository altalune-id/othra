<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';

export interface RotateSecretDialogLabels {
  title: string;
  body: string;
  grace: string;
  nowWarning: string;
  endsOld: string;
  cancel: string;
  confirm: string;
}

const { open, options, nowValue, oldHint = '', busy = false, error = null, labels = {
  title: 'Rotate the client secret?',
  body: 'You get a new secret now. The current one keeps working for the time you choose, so you can update the app first.',
  grace: 'Keep the current secret working for',
  nowWarning: 'Right away breaks apps that still use the current secret.',
  endsOld: 'The old secret …{hint} stops working now.',
  cancel: 'Cancel',
  confirm: 'Rotate secret',
} } = defineProps<{
  open: boolean,
  options: { value: number, label: string }[],
  nowValue: number,
  oldHint?: string,
  busy?: boolean,
  error?: ShownError | null,
  labels?: RotateSecretDialogLabels,
}>();

const emit = defineEmits<{ confirm: [], close: [] }>();
const grace = defineModel<number>('grace', { required: true });
const id = useId();
</script>

<template>
  <AppModal :open="open" :title="labels.title" @close="emit('close')">
    <div class="space-y-3">
      <p class="text-sm">
        {{ labels.body }}
      </p>
      <div class="fieldset">
        <label :for="id" class="fieldset-legend">{{ labels.grace }}</label>
        <select :id="id" v-model.number="grace" class="select w-full border-base-300 sm:w-56" :disabled="busy">
          <option v-for="o in options" :key="o.value" :value="o.value">
            {{ o.label }}
          </option>
        </select>
      </div>
      <p v-if="grace === nowValue" class="flex items-start gap-1 text-sm text-base-content">
        <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" />
        {{ labels.nowWarning }}
      </p>
      <p v-if="oldHint" class="flex items-start gap-1 text-sm text-base-content">
        <Icon name="lucide:info" size="16" class="mt-0.5 shrink-0 text-info" />
        {{ labels.endsOld.replace('{hint}', oldHint) }}
      </p>
      <ErrorText :error="error" />
    </div>
    <template #footer>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" autofocus @click="emit('close')">
        {{ labels.cancel }}
      </button>
      <button type="button" class="btn btn-primary btn-sm" :disabled="busy" @click="emit('confirm')">
        <span v-if="busy" class="loading loading-spinner loading-xs" />
        {{ labels.confirm }}
      </button>
    </template>
  </AppModal>
</template>
