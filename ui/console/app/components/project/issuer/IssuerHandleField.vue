<script setup lang="ts">
import type { HandleStatus } from './useIssuerHandle';
import { computed, useId } from 'vue';
import { handleProblem } from './useIssuerHandle';

export interface IssuerHandleLabels {
  handle: string;
  help: string;
  checking: string;
  available: string;
  taken: string;
  reserved: string;
  invalid: string;
  invalidFormat?: string;
  invalidLength?: string;
  locked: string;
  error: string;
}

const props = withDefaults(defineProps<{
  modelValue: string,
  status: HandleStatus,
  prefix?: string,
  disabled?: boolean,
  fieldError?: string,
  labels?: IssuerHandleLabels,
}>(), {
  prefix: '',
  disabled: false,
  fieldError: '',
  labels: () => ({
    handle: 'Issuer handle',
    help: 'Part of the issuer URL. 3–40 characters: lowercase letters, digits and hyphens, not at the start or end. Unique across Othra.',
    checking: 'Checking…',
    available: 'Available',
    taken: 'Already taken',
    reserved: 'Reserved',
    invalid: 'Not allowed',
    invalidFormat: 'Use lowercase letters, digits and hyphens, not at the start or end.',
    invalidLength: 'The issuer handle must be 3 to 40 characters.',
    locked: 'Locked',
    error: 'Couldn\'t check the handle',
  }),
});

const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

const id = useId();
const ids = { input: `${id}-handle`, help: `${id}-help`, status: `${id}-status`, error: `${id}-error` };
const locked = computed(() => props.status === 'locked');
const bad = computed(() => ['taken', 'reserved', 'invalid', 'error'].includes(props.status));
const invalidText = computed(() => {
  const problem = handleProblem(props.modelValue);
  const specific = problem === 'length' ? props.labels.invalidLength : problem === 'format' ? props.labels.invalidFormat : '';
  return specific || props.labels.invalid;
});
const statusText = computed(() => {
  switch (props.status) {
    case 'checking': return props.labels.checking;
    case 'available': return props.labels.available;
    case 'taken': return props.labels.taken;
    case 'reserved': return props.labels.reserved;
    case 'invalid': return invalidText.value;
    case 'error': return props.labels.error;
    default: return '';
  }
});
const describedBy = computed(() => [ids.help, statusText.value ? ids.status : '', props.fieldError ? ids.error : ''].filter(Boolean).join(' '));

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value);
}
</script>

<template>
  <div class="fieldset">
    <label class="flex flex-col gap-1">
      <span class="label text-ui text-base-content">{{ labels.handle }}</span>
      <span class="join w-full">
        <span v-if="prefix" class="join-item flex items-center border border-base-300 bg-base-200 px-3 font-mono text-xs text-base-content/70">{{ prefix }}</span>
        <input
          :id="ids.input"
          :value="modelValue"
          type="text"
          class="input input-sm join-item w-full font-mono text-ui"
          :class="{ 'input-error': bad || !!fieldError }"
          :readonly="locked"
          :disabled="disabled"
          autocomplete="off"
          spellcheck="false"
          maxlength="40"
          :aria-invalid="bad || !!fieldError"
          :aria-describedby="describedBy"
          @input="onInput"
        >
        <span v-if="locked" class="join-item flex items-center border border-base-300 bg-base-200 px-3" :title="labels.locked">
          <Icon name="lucide:lock" size="14" aria-hidden="true" />
          <span class="sr-only">{{ labels.locked }}</span>
        </span>
      </span>
    </label>
    <p v-if="statusText" :id="ids.status" class="flex items-center gap-1 text-xs" :class="bad ? 'text-base-content' : 'text-base-content/70'" aria-live="polite">
      <span v-if="status === 'checking'" class="loading loading-spinner loading-xs" aria-hidden="true" />
      <Icon v-else-if="status === 'available'" name="lucide:check" size="14" class="shrink-0 text-success" aria-hidden="true" />
      <Icon v-else name="lucide:x" size="14" class="shrink-0" :class="{ 'text-error': bad }" aria-hidden="true" />
      {{ statusText }}
    </p>
    <p v-if="fieldError" :id="ids.error" class="flex items-start gap-1 text-xs text-base-content">
      <Icon name="lucide:circle-alert" size="14" class="shrink-0 text-error" aria-hidden="true" />
      {{ fieldError }}
    </p>
    <p :id="ids.help" class="label whitespace-normal text-xs text-base-content/70">
      {{ labels.help }}
    </p>
  </div>
</template>
