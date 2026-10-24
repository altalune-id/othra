<script setup lang="ts">
import { useId } from 'vue';
import FieldError from '../ui/FieldError.vue';

export interface ScopeTableLabels {
  legend: string;
  help: string;
  scope: string;
  remove: string;
  add: string;
  description: string;
  descriptionPlaceholder: string;
}

const { errors = [], editable = true, disabled = false, full = false, labels = {
  legend: 'Scopes',
  help: 'Permissions apps can ask for, for example devices:read.',
  scope: 'Scope',
  remove: 'Remove scope',
  add: 'Add scope',
  description: 'What it lets the app do (optional)',
  descriptionPlaceholder: 'Read your messages',
} } = defineProps<{
  errors?: string[],
  editable?: boolean,
  disabled?: boolean,
  full?: boolean,
  labels?: ScopeTableLabels,
}>();

const emit = defineEmits<{ add: [], remove: [index: number], edit: [index: number] }>();
const rows = defineModel<string[]>('rows', { required: true });
const descriptions = defineModel<string[]>('descriptions', { default: () => [] });
const id = useId();

function set(index: number, value: string) {
  rows.value = rows.value.map((row, i) => (i === index ? value : row));
  emit('edit', index);
}

function setDescription(index: number, value: string) {
  descriptions.value = rows.value.map((_, i) => (i === index ? value : descriptions.value[i] ?? ''));
  emit('edit', index);
}
</script>

<template>
  <fieldset v-if="editable" class="fieldset">
    <legend class="fieldset-legend">
      {{ labels.legend }}
    </legend>
    <div class="hidden gap-2 text-xs font-medium text-base-content/70 sm:grid sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_2rem]" aria-hidden="true" data-testid="scope-columns">
      <span>{{ labels.scope }}</span>
      <span>{{ labels.description }}</span>
    </div>
    <div v-for="(row, i) in rows" :key="i" class="rounded-box border border-base-300 p-2 sm:border-0 sm:p-0" data-testid="scope-row">
      <div class="grid grid-cols-[minmax(0,1fr)_2rem] items-end gap-2 sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_2rem]">
        <div class="min-w-0">
          <label :for="`${id}-scope-${i}`" class="mb-1 block text-xs text-base-content/70 sm:sr-only">{{ labels.scope }}</label>
          <input
            :id="`${id}-scope-${i}`"
            :value="row"
            type="text"
            class="input w-full font-mono"
            :class="{ 'input-error': errors[i] }"
            :placeholder="i === 0 ? 'devices:read' : ''"
            :disabled="disabled"
            :aria-label="`${labels.scope} ${i + 1}`"
            :aria-invalid="!!errors[i]"
            :aria-describedby="`${id}-err-${i}`"
            autocomplete="off"
            spellcheck="false"
            @input="set(i, ($event.target as HTMLInputElement).value)"
          >
        </div>
        <div class="col-start-1 min-w-0 sm:col-start-2 sm:row-start-1">
          <label :for="`${id}-desc-${i}`" class="mb-1 block text-xs text-base-content/70 sm:sr-only">{{ labels.description }}</label>
          <input
            :id="`${id}-desc-${i}`"
            :value="descriptions[i] ?? ''"
            type="text"
            class="input w-full"
            maxlength="120"
            :placeholder="i === 0 ? labels.descriptionPlaceholder : ''"
            :disabled="disabled"
            :aria-label="`${labels.description} ${i + 1}`"
            :aria-describedby="`${id}-err-${i}`"
            @input="setDescription(i, ($event.target as HTMLInputElement).value)"
          >
        </div>
        <button
          v-if="rows.length > 1"
          type="button"
          class="btn btn-ghost btn-sm btn-square col-start-2 row-span-2 row-start-1 self-center sm:col-start-3 sm:row-span-1 sm:self-end"
          :disabled="disabled"
          :aria-label="`${labels.remove} ${i + 1}`"
          :title="`${labels.remove} ${i + 1}`"
          @click="emit('remove', i)"
        >
          <Icon name="lucide:x" size="16" />
        </button>
      </div>
      <FieldError :id="`${id}-err-${i}`" :message="errors[i]" />
    </div>
    <div v-if="!full">
      <button type="button" class="btn btn-ghost btn-sm" :disabled="disabled" @click="emit('add')">
        <Icon name="lucide:plus" size="16" />
        {{ labels.add }}
      </button>
    </div>
    <p class="label whitespace-normal">
      {{ labels.help }}
    </p>
  </fieldset>
  <ul v-else class="divide-y divide-base-300">
    <li v-for="(scope, i) in rows" :key="scope" class="py-1.5">
      <span class="badge badge-ghost badge-sm font-mono">{{ scope }}</span>
      <span v-if="descriptions[i]" class="mt-0.5 block text-sm text-base-content/70">{{ descriptions[i] }}</span>
    </li>
  </ul>
</template>
