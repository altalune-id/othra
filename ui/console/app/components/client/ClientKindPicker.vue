<script setup lang="ts">
import type { ClientKindOption } from './ClientForm.vue';
import type { ClientKindName } from './kinds';

const { kinds, modelValue, busy = false, legend, note } = defineProps<{
  kinds: ClientKindOption[],
  modelValue: ClientKindName,
  busy?: boolean,
  legend: string,
  note: string,
}>();

const emit = defineEmits<{ 'update:modelValue': [kind: ClientKindName] }>();
</script>

<template>
  <fieldset class="fieldset gap-2 p-0">
    <legend class="sr-only">
      {{ legend }}
    </legend>
    <div class="grid gap-2 sm:grid-cols-2">
      <label
        v-for="k in kinds"
        :key="k.value"
        class="flex cursor-pointer items-start gap-3 rounded-box border border-base-300 p-3 has-checked:border-primary has-checked:bg-base-200"
      >
        <input
          type="radio"
          name="client-kind"
          class="radio radio-sm mt-0.5"
          :value="k.value"
          :checked="modelValue === k.value"
          :disabled="busy"
          @change="emit('update:modelValue', k.value)"
        >
        <span class="min-w-0">
          <span class="flex items-center gap-2 text-sm font-medium">
            <Icon :name="k.icon" size="16" class="shrink-0" aria-hidden="true" />
            {{ k.label }}
          </span>
          <span class="block text-ui text-base-content/70">{{ k.help }}</span>
          <span class="block text-xs text-base-content/70">{{ k.rec }}</span>
        </span>
      </label>
    </div>
    <p class="text-xs text-base-content/70">
      {{ note }}
    </p>
  </fieldset>
</template>
