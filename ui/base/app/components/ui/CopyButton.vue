<script setup lang="ts">
import { ref } from 'vue';

const { value, label = 'Copy', text = '' } = defineProps<{ value: string, label?: string, text?: string }>();

const copied = ref(false);

async function copy() {
  try {
    await navigator.clipboard.writeText(value);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 1200);
  } catch {
    // clipboard unavailable
  }
}
</script>

<template>
  <button v-if="text" type="button" class="btn btn-sm text-ui" @click="copy">
    <Icon :name="copied ? 'lucide:check' : 'lucide:copy'" size="16" class="shrink-0" :class="copied ? 'text-success' : ''" aria-hidden="true" />
    {{ text }}
  </button>
  <button v-else type="button" class="btn btn-ghost btn-xs btn-square" :aria-label="label" :title="label" @click="copy">
    <Icon :name="copied ? 'lucide:check' : 'lucide:copy'" size="14" :class="copied ? 'text-success' : ''" />
  </button>
</template>
