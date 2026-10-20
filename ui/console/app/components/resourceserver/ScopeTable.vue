<script setup lang="ts">
import { useId } from 'vue';
import FieldError from '../ui/FieldError.vue';

export interface ScopeTableLabels {
  legend: string;
  help: string;
  scope: string;
  remove: string;
  add: string;
}

const { errors = [], editable = true, disabled = false, full = false, labels = {
  legend: 'Scopes',
  help: 'Permissions apps can ask for, for example devices:read.',
  scope: 'Scope',
  remove: 'Remove scope',
  add: 'Add scope',
} } = defineProps<{
  errors?: string[],
  editable?: boolean,
  disabled?: boolean,
  full?: boolean,
  labels?: ScopeTableLabels,
}>();

const emit = defineEmits<{ add: [], remove: [index: number], edit: [index: number] }>();
const rows = defineModel<string[]>('rows', { required: true });
const id = useId();

function set(index: number, value: string) {
  rows.value = rows.value.map((row, i) => (i === index ? value : row));
  emit('edit', index);
}
</script>

<template>
  <fieldset v-if="editable" class="fieldset">
    <legend class="fieldset-legend">
      {{ labels.legend }}
    </legend>
    <div v-for="(row, i) in rows" :key="i">
      <div class="flex items-center gap-2">
        <input
          :value="row"
          type="text"
          class="input w-full font-mono sm:max-w-sm"
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
        <button
          v-if="rows.length > 1"
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
  <ul v-else class="flex flex-wrap gap-1.5">
    <li v-for="scope in rows" :key="scope" class="badge badge-ghost badge-sm font-mono">
      {{ scope }}
    </li>
  </ul>
</template>
