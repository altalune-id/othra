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
  <fieldset class="fieldset">
    <legend class="fieldset-legend text-ui">
      {{ legend }}
    </legend>
    <div class="grid gap-3 sm:grid-cols-2">
      <label
        v-for="k in kinds"
        :key="k.value"
        class="flex cursor-pointer items-start gap-3 rounded-box border p-3"
        :class="modelValue === k.value ? 'border-primary' : 'border-base-300'"
      >
        <input
          type="radio"
          name="client-kind"
          class="radio radio-sm radio-primary mt-0.5"
          :value="k.value"
          :checked="modelValue === k.value"
          :disabled="busy"
          @change="emit('update:modelValue', k.value)"
        >
        <span class="space-y-0.5">
          <span class="flex items-center gap-1.5 text-sm font-medium">
            <Icon :name="k.icon" size="16" />
            {{ k.label }}
          </span>
          <span class="block text-xs text-base-content/70">{{ k.help }}</span>
        </span>
      </label>
    </div>
    <p class="label whitespace-normal text-xs text-base-content/70">
      {{ note }}
    </p>
  </fieldset>
</template>
