<script setup lang="ts">
import type { Column } from '#base/components/ui/types';
import AppBadge from '#base/components/ui/AppBadge.vue';
import AppDropdown from '#base/components/ui/AppDropdown.vue';
import { useCopy } from '#base/components/ui/copy';
import DataTable from '#base/components/ui/DataTable.vue';
import SystemBadge from '#base/components/ui/SystemBadge.vue';

export interface ClientRow {
  id: string;
  name: string;
  type: string;
  icon: string;
  host: string;
  lastUsed: string;
  to: string;
  system?: boolean;
}

export interface ClientListLabels {
  caption: string;
  name: string;
  type: string;
  clientId: string;
  host: string;
  lastUsed: string;
  actions: string;
  moreFor: string;
  open: string;
  copyId: string;
  copied: string;
  system: string;
}

const { rows, loading = false, labels } = defineProps<{ rows: ClientRow[], loading?: boolean, labels: ClientListLabels }>();
const { copy } = useCopy();

const columns = computed<Column[]>(() => [
  { key: 'name', label: labels.name },
  { key: 'type', label: labels.type },
  { key: 'id', label: labels.clientId, mono: true },
  { key: 'host', label: labels.host, mono: true },
  { key: 'lastUsed', label: labels.lastUsed },
  { key: 'menu', label: labels.actions, hideLabel: true, align: 'end' },
]);
</script>

<template>
  <DataTable :caption="labels.caption" row-key="id" :columns="columns" :rows="rows" :loading="loading">
    <template #cell-name="{ row }">
      <span class="inline-flex flex-wrap items-center gap-2">
        <NuxtLink :to="row.to" class="link link-hover font-medium">{{ row.name }}</NuxtLink>
        <SystemBadge v-if="row.system" :label="labels.system" />
      </span>
    </template>
    <template #cell-type="{ row }">
      <AppBadge :label="row.type" :icon="row.icon" />
    </template>
    <template #cell-id="{ row }">
      <span class="block max-w-44 truncate" :title="row.id">{{ row.id }}</span>
    </template>
    <template #cell-lastUsed="{ row }">
      <span class="tabular-nums text-base-content/70">{{ row.lastUsed }}</span>
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
            <button type="button" @click="copy(row.id, labels.copied)">
              {{ labels.copyId }}
            </button>
          </li>
        </ul>
      </AppDropdown>
    </template>
  </DataTable>
</template>
