<script setup lang="ts">
import type { HandleStatus } from './useIssuerHandle';
import { computed, useId } from 'vue';

export interface IssuerHandleLabels {
  handle: string;
  help: string;
  checking: string;
  available: string;
  taken: string;
  reserved: string;
  invalid: string;
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
    locked: 'Locked',
    error: 'Couldn\'t check the handle',
  }),
});

const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

const id = useId();
const ids = { input: `${id}-handle`, help: `${id}-help`, status: `${id}-status`, error: `${id}-error` };
const locked = computed(() => props.status === 'locked');
const bad = computed(() => ['taken', 'reserved', 'invalid', 'error'].includes(props.status));
const statusText = computed(() => {
  switch (props.status) {
    case 'checking': return props.labels.checking;
    case 'available': return props.labels.available;
    case 'taken': return props.labels.taken;
    case 'reserved': return props.labels.reserved;
    case 'invalid': return props.labels.invalid;
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
    <label :for="ids.input" class="fieldset-legend">{{ labels.handle }}</label>
    <div class="join w-full">
      <span v-if="prefix" class="join-item flex max-w-[55%] items-center truncate border border-base-300 bg-base-200 px-3 font-mono text-xs text-base-content/60" :title="prefix">{{ prefix }}</span>
      <input
        :id="ids.input"
        :value="modelValue"
        type="text"
        class="input join-item w-full font-mono"
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
        <Icon name="lucide:lock" size="14" />
        <span class="sr-only">{{ labels.locked }}</span>
      </span>
    </div>
    <p v-if="statusText" :id="ids.status" class="flex items-center gap-1 text-sm" :class="status === 'available' ? 'text-success' : bad ? 'text-error' : 'text-base-content/60'" aria-live="polite">
      <span v-if="status === 'checking'" class="loading loading-spinner loading-xs" />
      <Icon v-else-if="status === 'available'" name="lucide:check" size="14" />
      <Icon v-else name="lucide:x" size="14" />
      {{ statusText }}
    </p>
    <p v-if="fieldError" :id="ids.error" class="text-sm text-error">
      {{ fieldError }}
    </p>
    <p :id="ids.help" class="label whitespace-normal">
      {{ labels.help }}
    </p>
  </div>
</template>
