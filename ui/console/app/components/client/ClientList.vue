<script setup lang="ts">
import CopyButton from '#base/components/ui/CopyButton.vue';

export interface ClientRow {
  id: string;
  name: string;
  kind: string;
  created: string;
  to: string;
}

export interface ClientListLabels {
  name: string;
  clientId: string;
  kind: string;
  created: string;
  copyFor: string;
}

const { rows, loading = false, labels = {
  name: 'Name',
  clientId: 'Client ID',
  kind: 'Kind',
  created: 'Created',
  copyFor: 'Copy client ID for {name}',
} } = defineProps<{ rows: ClientRow[], loading?: boolean, labels?: ClientListLabels }>();
</script>

<template>
  <div>
    <div class="overflow-x-auto max-sm:hidden">
      <table class="table table-sm">
        <thead>
          <tr>
            <th>{{ labels.name }}</th>
            <th>{{ labels.clientId }}</th>
            <th>{{ labels.kind }}</th>
            <th>{{ labels.created }}</th>
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
            <td>
              <span class="inline-flex items-center gap-1">
                <code class="font-mono text-xs">{{ row.id }}</code>
                <CopyButton :value="row.id" :label="labels.copyFor.replace('{name}', row.name)" />
              </span>
            </td>
            <td>{{ row.kind }}</td>
            <td class="whitespace-nowrap">
              {{ row.created }}
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
          <NuxtLink :to="row.to" class="link link-hover font-medium">
            {{ row.name }}
          </NuxtLink>
          <span class="flex items-center gap-1">
            <code class="min-w-0 font-mono text-xs break-all">{{ row.id }}</code>
            <CopyButton :value="row.id" :label="labels.copyFor.replace('{name}', row.name)" />
          </span>
          <p class="text-xs text-base-content/70">
            {{ row.kind }} · {{ row.created }}
          </p>
        </li>
      </template>
    </ul>
  </div>
</template>
