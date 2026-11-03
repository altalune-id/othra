<script setup lang="ts">
import type { Column, StatusTone } from '#base/components/ui/types';
import type { KeyRow, KeyStatus } from './keyRows';
import CopyButton from '#base/components/ui/CopyButton.vue';
import DataTable from '#base/components/ui/DataTable.vue';
import StatusLabel from '#base/components/ui/StatusLabel.vue';

export interface SigningKeyListLabels {
  caption: string;
  kid: string;
  alg: string;
  status: string;
  created: string;
  retires: string;
  next: string;
  active: string;
  retiring: string;
  retired: string;
  unknown: string;
  copy: string;
  retiredNote: string;
}

const { rows, loading = false, labels = {
  caption: 'Signing keys',
  kid: 'Key ID',
  alg: 'Algorithm',
  status: 'Status',
  created: 'Created',
  retires: 'Retires',
  next: 'Next',
  active: 'Active',
  retiring: 'Retiring',
  retired: 'Retired',
  unknown: '—',
  copy: 'Copy',
  retiredNote: 'Retired keys are no longer published and are removed from this list after 30 days.',
} } = defineProps<{ rows: KeyRow[], loading?: boolean, labels?: SigningKeyListLabels }>();

const tones: Record<KeyStatus, StatusTone> = { next: 'info', active: 'success', retiring: 'warning', retired: 'neutral', unknown: 'neutral' };

const columns = computed<Column[]>(() => [
  { key: 'kid', label: labels.kid },
  { key: 'alg', label: labels.alg },
  { key: 'status', label: labels.status },
  { key: 'created', label: labels.created },
  { key: 'retires', label: labels.retires },
]);

function dim(row: KeyRow): string {
  return row.status === 'retired' ? 'text-base-content/70' : '';
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <DataTable :caption="labels.caption" row-key="id" :columns="columns" :rows="rows" :loading="loading" :row-class="dim">
      <template #cell-kid="{ row }">
        <span class="inline-flex items-center gap-1">
          <code class="font-mono text-xs" :title="row.kid">{{ row.short }}…</code>
          <CopyButton :value="row.kid" :label="`${labels.copy} ${labels.kid}`" />
        </span>
      </template>
      <template #cell-status="{ row }">
        <span :title="row.status === 'retired' ? labels.retiredNote : undefined">
          <StatusLabel :tone="tones[row.status]" :label="labels[row.status]" />
        </span>
        <p v-if="row.help" class="text-xs text-base-content/70">
          {{ row.help }}
        </p>
      </template>
      <template #cell-created="{ row }">
        <span class="whitespace-nowrap tabular-nums">{{ row.created }}</span>
      </template>
      <template #cell-retires="{ row }">
        <span v-if="row.retires" class="whitespace-nowrap tabular-nums">{{ row.retires }}</span>
        <span v-else class="text-base-content/70">—</span>
      </template>
    </DataTable>
    <p v-if="!loading && rows.some(r => r.status === 'retired')" class="text-xs text-base-content/70">
      {{ labels.retiredNote }}
    </p>
  </div>
</template>
