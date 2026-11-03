<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import type { VerificationName } from '../resourceserver/verification';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import VerificationStatus from '../resourceserver/VerificationStatus.vue';

export interface LinkRow {
  id: string;
  name: string;
  detail: string;
  scopes: string[];
  status?: { name: VerificationName, label: string };
}

export interface LinkListLabels {
  title: string;
  link: string;
  empty: string;
  emptyReadOnly: string;
  edit: string;
  unlink: string;
}

const { rows, canEdit = false, canLink = true, loading = false, loadError = null, labels = {
  title: 'Resource servers',
  link: 'Link resource server',
  empty: 'Not linked yet. Link one so it can ask for its scopes.',
  emptyReadOnly: 'Not linked yet.',
  edit: 'Edit',
  unlink: 'Unlink',
} } = defineProps<{
  rows: LinkRow[],
  canEdit?: boolean,
  canLink?: boolean,
  loading?: boolean,
  loadError?: ShownError | null,
  labels?: LinkListLabels,
}>();

const emit = defineEmits<{ link: [], edit: [row: LinkRow], unlink: [row: LinkRow] }>();
</script>

<template>
  <section class="flex min-w-0 flex-col gap-3 rounded-box border border-base-300 bg-base-100 p-4" data-testid="links-card">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h2 class="text-ui font-semibold">
        {{ labels.title }}
      </h2>
      <AppButton v-if="canEdit && canLink && !loading" variant="plain" size="sm" class="max-sm:btn-block" data-testid="link-button" @click="emit('link')">
        <Icon name="lucide:link" size="16" />
        {{ labels.link }}
      </AppButton>
    </div>
    <ErrorText v-if="loadError" :error="loadError" />
    <div v-else-if="loading" class="skeleton h-4 w-48" />
    <p v-else-if="rows.length === 0" class="text-ui text-base-content/70">
      {{ canEdit ? labels.empty : labels.emptyReadOnly }}
    </p>
    <ul v-else class="divide-y divide-base-300">
      <li v-for="row in rows" :key="row.id" class="flex flex-wrap items-start justify-between gap-2 py-2">
        <div class="min-w-0 space-y-1">
          <p class="flex flex-wrap items-center gap-x-3 gap-y-1 text-ui">
            <span class="font-medium">{{ row.name }}</span>
            <VerificationStatus v-if="row.status" :name="row.status.name" :label="row.status.label" />
          </p>
          <p class="font-mono text-xs break-all text-base-content/70">
            {{ row.detail }}
          </p>
          <ul class="flex flex-wrap gap-1">
            <li v-for="scope in row.scopes" :key="scope" class="badge badge-ghost badge-sm font-mono">
              {{ scope }}
            </li>
          </ul>
        </div>
        <div v-if="canEdit" class="flex shrink-0 gap-1">
          <AppButton variant="ghost" size="xs" @click="emit('edit', row)">
            <Icon name="lucide:pencil" size="14" class="shrink-0" aria-hidden="true" />
            {{ labels.edit }}
          </AppButton>
          <AppButton variant="ghost" size="xs" @click="emit('unlink', row)">
            <Icon name="lucide:unlink" size="14" class="shrink-0 text-error" aria-hidden="true" />
            {{ labels.unlink }}
          </AppButton>
        </div>
      </li>
    </ul>
  </section>
</template>
