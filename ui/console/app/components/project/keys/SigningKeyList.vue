<script setup lang="ts">
import type { KeyRow, KeyStatus } from './keyRows';
import CopyButton from '../../ui/CopyButton.vue';

export interface SigningKeyListLabels {
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

function dot(status: KeyStatus): string {
  switch (status) {
    case 'next': return 'status-info';
    case 'active': return 'status-success';
    case 'retiring': return 'status-warning';
    default: return 'hidden';
  }
}
</script>

<template>
  <div class="space-y-2">
    <table class="table table-sm max-sm:hidden">
      <thead>
        <tr>
          <th>{{ labels.kid }}</th>
          <th>{{ labels.alg }}</th>
          <th>{{ labels.status }}</th>
          <th>{{ labels.created }}</th>
          <th>{{ labels.retires }}</th>
        </tr>
      </thead>
      <tbody v-if="loading">
        <tr v-for="n in 3" :key="n">
          <td v-for="c in 5" :key="c">
            <div class="skeleton h-4 w-24" />
          </td>
        </tr>
      </tbody>
      <tbody v-else>
        <tr v-for="row in rows" :key="row.id" :class="{ 'text-base-content/60': row.status === 'retired' }">
          <td>
            <span class="inline-flex items-center gap-1">
              <code class="font-mono text-xs" :title="row.kid">{{ row.short }}…</code>
              <CopyButton :value="row.kid" :label="`${labels.copy} ${labels.kid}`" />
            </span>
          </td>
          <td>{{ row.alg }}</td>
          <td>
            <span class="inline-flex items-center gap-1.5 text-sm" :title="row.status === 'retired' ? labels.retiredNote : undefined">
              <span class="status" :class="dot(row.status)" aria-hidden="true" />
              {{ labels[row.status] }}
            </span>
            <p v-if="row.help" class="text-xs text-base-content/60">
              {{ row.help }}
            </p>
          </td>
          <td>{{ row.created }}</td>
          <td>
            <template v-if="row.retires">
              {{ row.retires }}
            </template>
            <span v-else class="text-base-content/50">—</span>
          </td>
        </tr>
      </tbody>
    </table>

    <ul class="space-y-2 sm:hidden">
      <template v-if="loading">
        <li v-for="n in 3" :key="n" class="rounded-box border border-base-300 p-3">
          <div class="skeleton h-4 w-32" />
        </li>
      </template>
      <template v-else>
        <li v-for="row in rows" :key="row.id" class="space-y-1 rounded-box border border-base-300 p-3" :class="{ 'text-base-content/60': row.status === 'retired' }">
          <div class="flex items-center justify-between gap-2">
            <span class="inline-flex items-center gap-1">
              <code class="font-mono text-xs" :title="row.kid">{{ row.short }}…</code>
              <CopyButton :value="row.kid" :label="`${labels.copy} ${labels.kid}`" />
            </span>
            <span class="inline-flex items-center gap-1.5 text-sm">
              <span class="status" :class="dot(row.status)" aria-hidden="true" />
              {{ labels[row.status] }}
            </span>
          </div>
          <p v-if="row.help" class="text-xs text-base-content/60">
            {{ row.help }}
          </p>
          <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-sm">
            <dt class="text-base-content/60">
              {{ labels.alg }}
            </dt>
            <dd>{{ row.alg }}</dd>
            <dt class="text-base-content/60">
              {{ labels.created }}
            </dt>
            <dd>{{ row.created }}</dd>
            <dt class="text-base-content/60">
              {{ labels.retires }}
            </dt>
            <dd>
              <template v-if="row.retires">
                {{ row.retires }}
              </template>
              <span v-else class="text-base-content/50">—</span>
            </dd>
          </dl>
        </li>
      </template>
    </ul>

    <p v-if="!loading && rows.some(r => r.status === 'retired')" class="text-xs text-base-content/60">
      {{ labels.retiredNote }}
    </p>
  </div>
</template>
