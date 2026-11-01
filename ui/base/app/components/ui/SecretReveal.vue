<script setup lang="ts">
import type { ShownError } from '../../lib/apperror/message';
import { computed, ref, useId } from 'vue';
import CopyButton from './CopyButton.vue';
import ErrorText from './ErrorText.vue';
import SecretField from './SecretField.vue';

const {
  value = '',
  label = 'Copy',
  warning,
  confirmLabel,
  envName = '',
  env,
  envLabel = 'Copy as .env',
  error = null,
} = defineProps<{
  value?: string,
  label?: string,
  warning: string,
  confirmLabel: string,
  envName?: string,
  env?: Record<string, string>,
  envLabel?: string,
  error?: ShownError | null,
}>();

const emit = defineEmits<{ confirmed: [value: boolean] }>();

const checked = ref(false);
const checkId = useId();

const envText = computed(() => {
  const vars = env ?? (envName ? { [envName]: value } : {});
  return Object.entries(vars).map(([k, v]) => `${k}=${v}`).join('\n');
});

function toggle(e: Event) {
  checked.value = (e.target as HTMLInputElement).checked;
  emit('confirmed', checked.value);
}
</script>

<template>
  <div v-if="error" role="alert" class="alert alert-error alert-soft flex flex-wrap items-start justify-between gap-3">
    <ErrorText :error="error" />
    <slot name="retry" />
  </div>
  <div v-else class="space-y-3">
    <p class="alert alert-warning alert-soft items-start text-sm">
      <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0" />
      <span>{{ warning }}</span>
    </p>
    <SecretField :value="value" :label="label" />
    <div v-if="envText" class="flex items-center gap-2 text-sm">
      <span>{{ envLabel }}</span>
      <CopyButton :value="envText" :label="envLabel" />
    </div>
    <label :for="checkId" class="flex items-center gap-2 text-sm">
      <input :id="checkId" type="checkbox" class="checkbox checkbox-sm rounded-sm" :checked="checked" @change="toggle">
      {{ confirmLabel }}
    </label>
  </div>
</template>
