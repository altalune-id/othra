<script setup lang="ts">
import type { Ref } from 'vue';
import type { ShownError } from '#base/lib/apperror/message';
import type { ToolCaller } from './bridge';
import type { ListDescriptor } from './descriptor';
import ErrorText from '#base/components/ui/ErrorText.vue';
import ViewShell from '../components/ViewShell.vue';
import { t } from '../copy';
import { callTool } from './bridge';
import { pick, rowsAt, valueAt } from './descriptor';
import RowTable from './RowTable.vue';

const { data, descriptor: d } = defineProps<{ data: Record<string, unknown>, descriptor: ListDescriptor }>();
const app = inject<ToolCaller>('mcpApp');
const input = inject<Ref<Record<string, unknown>>>('mcpInput', ref({}));

const rows = ref(rowsAt(data, d.rows));
const next = ref(String(valueAt(data, d.more?.next) ?? ''));
const busy = ref(false);
const error = ref<ShownError | null>(null);

watch(() => data, (fresh) => {
  rows.value = rowsAt(fresh, d.rows);
  next.value = String(valueAt(fresh, d.more?.next) ?? '');
});

async function load(more: boolean) {
  if (!app || !d.refresh || busy.value) {
    return;
  }
  busy.value = true;
  error.value = null;
  try {
    const args = pick(input.value, d.refresh.args);
    if (more && d.more) {
      args[d.more.arg] = next.value;
    }
    const out = await callTool(app, d.refresh.tool, args);
    rows.value = more ? [...rows.value, ...rowsAt(out, d.rows)] : rowsAt(out, d.rows);
    next.value = String(valueAt(out, d.more?.next) ?? '');
  } catch (err) {
    error.value = { message: (err as Error).message };
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <ViewShell :title="d.title" :count="rows.length">
    <RowTable :rows="rows" :columns="d.columns" :row-id="d.key" :actions="d.actions" :empty="d.empty" @done="load(false)" />
    <div v-if="d.refresh" class="flex gap-2">
      <button class="btn btn-ghost btn-sm" type="button" :disabled="busy" @click="load(false)">
        <span v-if="busy" class="loading loading-spinner loading-xs" aria-hidden="true" />
        {{ busy ? t('mcp.refreshing') : t('common.refresh') }}
      </button>
      <button v-if="d.more && next" class="btn btn-ghost btn-sm" type="button" :disabled="busy" @click="load(true)">
        {{ t('endusers.more') }}
      </button>
    </div>
    <ErrorText :error="error" />
  </ViewShell>
</template>
