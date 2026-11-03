<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
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
      <p class="text-ui">
        {{ labels.body }}
      </p>
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ labels.grace }}</span>
          <select :id="id" v-model.number="grace" class="select select-sm w-full sm:w-56 text-ui" :disabled="busy">
            <option v-for="o in options" :key="o.value" :value="o.value">
              {{ o.label }}
            </option>
          </select>
        </label>
      </div>
      <p v-if="grace === nowValue" class="flex items-start gap-1 text-ui text-base-content">
        <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
        {{ labels.nowWarning }}
      </p>
      <p v-if="oldHint" class="flex items-start gap-1 text-ui text-base-content">
        <Icon name="lucide:info" size="16" class="mt-0.5 shrink-0 text-info" aria-hidden="true" />
        {{ labels.endsOld.replace('{hint}', oldHint) }}
      </p>
      <ErrorText :error="error" />
    </div>
    <template #footer>
      <AppButton variant="ghost" size="sm" :disabled="busy" autofocus @click="emit('close')">
        {{ labels.cancel }}
      </AppButton>
      <AppButton variant="primary" size="sm" :busy="busy" @click="emit('confirm')">
        {{ labels.confirm }}
      </AppButton>
    </template>
  </AppModal>
</template>
