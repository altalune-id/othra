<script setup lang="ts">
import type { Ref } from 'vue';
import type { ToolCaller } from './bridge';
import type { Field, RowAction } from './descriptor';
import { confirmHandle } from '#base/components/ui/confirmHandle';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import { t } from '../copy';
import { callTool } from './bridge';
import { badgeFor, formatValue, rowKey, valueAt } from './descriptor';
import FieldValue from './FieldValue.vue';

const { rows, columns, rowId, actions = [], empty } = defineProps<{
  rows: Record<string, unknown>[],
  columns: Field[],
  rowId: string | string[],
  actions?: RowAction[],
  empty: string,
}>();
const emit = defineEmits<{ done: [] }>();
const app = inject<ToolCaller>('mcpApp');
const input = inject<Ref<Record<string, unknown>>>('mcpInput', ref({}));

interface Pending {
  action: RowAction;
  row: Record<string, unknown>;
}

const handle = confirmHandle<Pending>(async ({ action, row }) => {
  if (!app) {
    return;
  }
  await callTool(app, action.tool, action.args(row, input.value));
}, err => ({ message: (err as Error).message }));

async function confirm(): Promise<boolean> {
  const ok = await handle.confirm();
  if (ok) {
    emit('done');
  }
  return ok;
}

const pending = computed(() => handle.target.value);
const shown = (row: Record<string, unknown>) => actions.filter(a => !a.when || a.when(row));
function rowName(row: Record<string, unknown>): string {
  const first = columns[0];
  if (!first) {
    return rowKey(row, rowId);
  }
  const value = valueAt(row, first.key);
  return first.kind === 'badge' ? badgeFor(first, value).label : formatValue(first, value);
}
</script>

<template>
  <p v-if="rows.length === 0" class="text-sm text-base-content/60">
    {{ empty }}
  </p>
  <div v-else class="overflow-x-auto">
    <table class="table table-sm">
      <thead>
        <tr>
          <th v-for="c in columns" :key="c.key">
            {{ c.label }}
          </th>
          <th v-if="actions.length">
            <span class="sr-only">{{ t('mcp.actions') }}</span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="rowKey(row, rowId)">
          <td v-for="c in columns" :key="c.key">
            <FieldValue :field="c" :value="valueAt(row, c.key)" />
          </td>
          <td v-if="actions.length" class="text-right whitespace-nowrap">
            <button
              v-for="a in shown(row)"
              :key="a.label"
              type="button"
              class="btn btn-ghost btn-xs text-error"
              :aria-label="`${a.label}: ${rowName(row)}`"
              @click="handle.ask({ action: a, row })"
            >
              {{ a.label }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <ConfirmModal
    :handle="{ ...handle, confirm }"
    :title="pending ? pending.action.confirm.title(pending.row) : ''"
    :body="pending ? pending.action.confirm.body(pending.row) : ''"
    :confirm-label="pending?.action.confirm.button ?? ''"
    :cancel-label="t('common.cancel')"
  />
</template>
