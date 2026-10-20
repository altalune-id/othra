<script setup lang="ts">
import { useId } from 'vue';
import FieldError from '../ui/FieldError.vue';

export interface RedirectUriListLabels {
  uri: string;
  remove: string;
  add: string;
  localhostWarning: string;
}

const { label, help = '', errors = [], warnings = [], required = true, disabled = false, max = 20, placeholder = '', labels = {
  uri: 'Redirect URI',
  remove: 'Remove redirect URI',
  add: 'Add URI',
  localhostWarning: '127.0.0.1 is safer than localhost, which some systems resolve elsewhere.',
} } = defineProps<{
  label: string,
  help?: string,
  errors?: string[],
  warnings?: boolean[],
  required?: boolean,
  disabled?: boolean,
  max?: number,
  placeholder?: string,
  labels?: RedirectUriListLabels,
}>();

const emit = defineEmits<{ edit: [index: number], remove: [index: number] }>();
const rows = defineModel<string[]>('rows', { required: true });
const id = useId();

function set(index: number, value: string) {
  rows.value = rows.value.map((row, i) => (i === index ? value : row));
  emit('edit', index);
}

function add() {
  rows.value = [...rows.value, ''];
}
</script>

<template>
  <fieldset class="fieldset">
    <legend class="fieldset-legend">
      {{ label }}
    </legend>
    <div v-for="(row, i) in rows" :key="i">
      <div class="flex items-center gap-2">
        <input
          :value="row"
          type="url"
          class="input w-full font-mono text-sm"
          :class="{ 'input-error': errors[i] }"
          :placeholder="i === 0 ? placeholder : ''"
          :disabled="disabled"
          :aria-label="`${labels.uri} ${i + 1}`"
          :aria-invalid="!!errors[i]"
          :aria-describedby="`${id}-err-${i}`"
          autocomplete="off"
          spellcheck="false"
          @input="set(i, ($event.target as HTMLInputElement).value)"
        >
        <button
          v-if="rows.length > 1 || !required"
          type="button"
          class="btn btn-ghost btn-sm btn-square shrink-0"
          :disabled="disabled"
          :aria-label="`${labels.remove} ${i + 1}`"
          :title="`${labels.remove} ${i + 1}`"
          @click="emit('remove', i)"
        >
          <Icon name="lucide:x" size="16" />
        </button>
      </div>
      <FieldError :id="`${id}-err-${i}`" :message="errors[i]" />
      <p v-if="warnings[i] && !errors[i]" class="mt-1 flex items-start gap-1 text-xs text-base-content">
        <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" />
        {{ labels.localhostWarning }}
      </p>
    </div>
    <div v-if="rows.length < max">
      <button type="button" class="btn btn-ghost btn-sm" :disabled="disabled" @click="add">
        <Icon name="lucide:plus" size="16" />
        {{ labels.add }}
      </button>
    </div>
    <p v-if="help" class="label whitespace-normal">
      {{ help }}
    </p>
  </fieldset>
</template>
