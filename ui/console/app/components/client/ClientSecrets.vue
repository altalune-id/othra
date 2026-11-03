<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { ref } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';

export interface SecretView {
  id: string;
  hint: string;
  old: boolean;
  created: string;
  expires: string;
}

export interface ClientSecretsLabels {
  title: string;
  intro: string;
  current: string;
  old: string;
  created: string;
  noSecret: string;
  generate: string;
  rotate: string;
  revoke: string;
  personOnly: string;
}

const { rows, canEdit = false, busy = false, error = null, labels = {
  title: 'Secrets',
  intro: 'Apps send the secret with the client ID. You can\'t see a secret after it\'s created.',
  current: 'Current',
  old: 'Old, works until {date}',
  created: 'Created {date}',
  noSecret: 'No secret yet. This client can\'t sign in until you generate one.',
  generate: 'Generate secret',
  rotate: 'Rotate secret',
  revoke: 'Revoke old secret now',
  personOnly: 'Only a signed-in person can create or rotate a secret.',
} } = defineProps<{
  rows: SecretView[],
  canEdit?: boolean,
  busy?: boolean,
  error?: ShownError | null,
  labels?: ClientSecretsLabels,
}>();

const emit = defineEmits<{ generate: [], rotate: [], revoke: [row: SecretView] }>();
const title = ref<HTMLElement>();
const action = ref<HTMLButtonElement>();

defineExpose({
  focusTitle: () => title.value?.focus(),
  focusAction: () => action.value?.focus(),
});
</script>

<template>
  <section class="flex min-w-0 flex-col gap-3 rounded-box border border-base-300 bg-base-100 p-4" data-testid="secrets-card">
    <h2 ref="title" tabindex="-1" class="text-ui font-semibold outline-none">
      {{ labels.title }}
    </h2>
    <p class="text-ui text-base-content/70">
      {{ labels.intro }}
    </p>
    <p v-if="rows.length === 0" class="flex items-start gap-1.5 text-ui text-base-content">
      <Icon name="lucide:key-round" size="16" class="mt-0.5 shrink-0 text-warning" />
      {{ labels.noSecret }}
    </p>
    <ul v-else class="divide-y divide-base-300 rounded-box border border-base-300">
      <li v-for="row in rows" :key="row.id" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2 text-ui">
        <code class="font-mono text-xs">••••{{ row.hint }}</code>
        <span class="inline-flex items-center gap-1.5">
          <span class="status" :class="row.old ? 'status-warning' : 'status-success'" aria-hidden="true" />
          {{ row.old ? labels.old.replace('{date}', row.expires) : labels.current }}
        </span>
        <span v-if="!row.old && row.created" class="text-base-content/70">{{ labels.created.replace('{date}', row.created) }}</span>
        <AppButton v-if="canEdit && row.old" variant="ghost" size="xs" class="ml-auto" :disabled="busy" @click="emit('revoke', row)">
          <Icon name="lucide:ban" size="14" class="shrink-0 text-error" aria-hidden="true" />
          {{ labels.revoke }}
        </AppButton>
      </li>
    </ul>
    <template v-if="canEdit">
      <div>
        <button ref="action" type="button" class="btn btn-sm max-sm:btn-block text-ui" :class="{ 'btn-primary': rows.length === 0 }" :disabled="busy" @click="rows.length === 0 ? emit('generate') : emit('rotate')">
          <span v-if="busy" class="loading loading-spinner loading-xs" />
          <Icon v-else :name="rows.length === 0 ? 'lucide:key-round' : 'lucide:refresh-cw'" size="16" />
          {{ rows.length === 0 ? labels.generate : labels.rotate }}
        </button>
      </div>
      <ErrorText :error="error" />
      <p class="text-xs text-base-content/70">
        {{ labels.personOnly }}
      </p>
    </template>
  </section>
</template>
