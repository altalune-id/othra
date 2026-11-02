<script setup lang="ts">
import type { Column } from '#base/components/ui/types';
import type { VerificationName } from './verification';
import AppDropdown from '#base/components/ui/AppDropdown.vue';
import { useCopy } from '#base/components/ui/copy';
import DataTable from '#base/components/ui/DataTable.vue';
import VerificationStatus from './VerificationStatus.vue';

export interface ResourceServerRow {
  id: string;
  name: string;
  identifier: string;
  status: VerificationName;
  statusLabel: string;
  checked: string;
  checkedTitle: string;
  to: string;
}

export interface ResourceServerListLabels {
  caption: string;
  name: string;
  identifier: string;
  status: string;
  checked: string;
  actions: string;
  moreFor: string;
  open: string;
  copyUri: string;
  copied: string;
}

const { rows, loading = false, labels = {
  caption: 'Resource servers',
  name: 'Name',
  identifier: 'Identifier',
  status: 'Verification',
  checked: 'Checked',
  actions: 'Actions',
  moreFor: 'More actions for {name}',
  open: 'Open',
  copyUri: 'Copy identifier',
  copied: 'Copied',
} } = defineProps<{ rows: ResourceServerRow[], loading?: boolean, labels?: ResourceServerListLabels }>();
const { copy } = useCopy();

const columns = computed<Column[]>(() => [
  { key: 'name', label: labels.name },
  { key: 'identifier', label: labels.identifier, mono: true },
  { key: 'status', label: labels.status },
  { key: 'checked', label: labels.checked },
  { key: 'menu', label: labels.actions, hideLabel: true, align: 'end' },
]);
</script>

<template>
  <DataTable :caption="labels.caption" row-key="id" :columns="columns" :rows="rows" :loading="loading">
    <template #cell-name="{ row }">
      <NuxtLink :to="row.to" class="link link-hover font-medium">
        {{ row.name }}
      </NuxtLink>
    </template>
    <template #cell-identifier="{ row }">
      <span class="block max-w-72 truncate" :title="row.identifier">{{ row.identifier }}</span>
    </template>
    <template #cell-status="{ row }">
      <VerificationStatus :name="row.status" :label="row.statusLabel" />
    </template>
    <template #cell-checked="{ row }">
      <span class="whitespace-nowrap tabular-nums text-base-content/70" :title="row.checkedTitle || undefined">{{ row.checked || '—' }}</span>
    </template>
    <template #cell-menu="{ row }">
      <AppDropdown placement="end" size="xs" trigger-class="btn-square" content-class="w-48" :label="labels.moreFor.replace('{name}', row.name)">
        <template #trigger>
          <Icon name="lucide:ellipsis" size="14" class="shrink-0" aria-hidden="true" />
        </template>
        <ul class="menu menu-sm w-full p-0 text-ui">
          <li>
            <NuxtLink :to="row.to">
              {{ labels.open }}
            </NuxtLink>
          </li>
          <li>
            <button type="button" @click="copy(row.identifier, labels.copied)">
              {{ labels.copyUri }}
            </button>
          </li>
        </ul>
      </AppDropdown>
    </template>
  </DataTable>
</template>
