<script setup lang="ts" generic="T extends object">
import type { Column } from './types';
import { useSlots } from 'vue';

const { columns, rows, rowKey, caption, loading = false } = defineProps<{
  columns: Column[],
  rows: T[],
  rowKey: keyof T & string,
  caption: string,
  loading?: boolean,
}>();

const slots = useSlots();

function cell(row: T, key: string): unknown {
  return (row as Record<string, unknown>)[key];
}
</script>

<template>
  <div>
    <div v-if="!loading && !rows.length && slots.empty">
      <slot name="empty" />
    </div>
    <template v-else>
      <div class="max-w-full overflow-x-auto rounded-box border border-base-300 bg-base-100">
        <table class="table table-sm text-ui" :aria-busy="loading || undefined">
          <caption class="sr-only">
            {{ caption }}
          </caption>
          <thead class="text-base-content/70">
            <tr>
              <th v-for="col in columns" :key="col.key" scope="col" class="bg-base-200 text-xs font-medium" :class="{ 'text-end': col.align === 'end' }">
                <span :class="{ 'sr-only': col.hideLabel }">{{ col.label }}</span>
              </th>
            </tr>
          </thead>
          <tbody v-if="loading">
            <tr v-for="n in 3" :key="n">
              <td v-for="col in columns" :key="col.key">
                <div class="skeleton h-4 w-24" />
              </td>
            </tr>
          </tbody>
          <tbody v-else>
            <tr v-for="row in rows" :key="String(cell(row, rowKey))">
              <td v-for="col in columns" :key="col.key" :class="{ 'text-end tabular-nums': col.align === 'end', 'font-mono text-xs': col.mono }">
                <slot :name="`cell-${col.key}`" :row="row">
                  {{ cell(row, col.key) }}
                </slot>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
