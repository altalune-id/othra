<script setup lang="ts">
import type { KeyValueItem } from './types';
import CopyButton from './CopyButton.vue';

const { items, copyLabel = 'Copy' } = defineProps<{ items: KeyValueItem[], copyLabel?: string }>();
</script>

<template>
  <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[minmax(9rem,auto)_1fr]">
    <template v-for="(item, i) in items" :key="i">
      <dt class="text-base-content/70">
        {{ item.label }}
      </dt>
      <dd class="flex min-w-0 items-center gap-1 break-words">
        <slot :name="`value-${i}`" :item="item">
          <span :class="{ 'font-mono text-xs break-all': item.mono }">{{ item.value || '—' }}</span>
          <CopyButton v-if="item.copy && item.value" :value="item.value" :label="`${copyLabel} ${item.label}`" class="shrink-0" />
        </slot>
      </dd>
    </template>
  </dl>
</template>
