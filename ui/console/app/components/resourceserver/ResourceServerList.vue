<script setup lang="ts">
import type { VerificationName } from './verification';
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
  name: string;
  identifier: string;
  status: string;
  checked: string;
}

const { rows, loading = false, labels = {
  name: 'Name',
  identifier: 'Identifier',
  status: 'Verification',
  checked: 'Checked',
} } = defineProps<{ rows: ResourceServerRow[], loading?: boolean, labels?: ResourceServerListLabels }>();
</script>

<template>
  <div>
    <div class="overflow-x-auto max-sm:hidden">
      <table class="table table-sm">
        <thead>
          <tr>
            <th>{{ labels.name }}</th>
            <th>{{ labels.identifier }}</th>
            <th>{{ labels.status }}</th>
            <th>{{ labels.checked }}</th>
          </tr>
        </thead>
        <tbody v-if="loading">
          <tr v-for="n in 3" :key="n">
            <td v-for="c in 4" :key="c">
              <div class="skeleton h-4 w-24" />
            </td>
          </tr>
        </tbody>
        <tbody v-else>
          <tr v-for="row in rows" :key="row.id">
            <td>
              <NuxtLink :to="row.to" class="link link-hover font-medium">
                {{ row.name }}
              </NuxtLink>
            </td>
            <td class="font-mono text-xs break-all">
              {{ row.identifier }}
            </td>
            <td>
              <VerificationStatus :name="row.status" :label="row.statusLabel" />
            </td>
            <td class="whitespace-nowrap" :title="row.checkedTitle || undefined">
              <span v-if="row.checked">{{ row.checked }}</span>
              <span v-else class="text-base-content/60">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <ul class="divide-y divide-base-300 sm:hidden">
      <template v-if="loading">
        <li v-for="n in 3" :key="n" class="py-3">
          <div class="skeleton h-4 w-32" />
        </li>
      </template>
      <template v-else>
        <li v-for="row in rows" :key="row.id" class="space-y-1 py-3">
          <div class="flex items-center justify-between gap-2">
            <NuxtLink :to="row.to" class="link link-hover font-medium">
              {{ row.name }}
            </NuxtLink>
            <VerificationStatus :name="row.status" :label="row.statusLabel" />
          </div>
          <p class="font-mono text-xs break-all">
            {{ row.identifier }}
          </p>
          <p v-if="row.checked" class="text-xs text-base-content/70" :title="row.checkedTitle || undefined">
            {{ labels.checked }} {{ row.checked }}
          </p>
        </li>
      </template>
    </ul>
  </div>
</template>
