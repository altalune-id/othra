<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { ref } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import Section from '#base/components/ui/Section.vue';
import StatusLabel from '#base/components/ui/StatusLabel.vue';

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
const action = ref<HTMLButtonElement>();

defineExpose({
  focusAction: () => action.value?.focus(),
});
</script>

<template>
  <Section :title="labels.title" :description="labels.intro" data-testid="secrets-card">
    <p v-if="rows.length === 0" class="flex items-start gap-1.5 text-ui text-base-content">
      <Icon name="lucide:key-round" size="16" class="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
      {{ labels.noSecret }}
    </p>
    <ul v-else class="divide-y divide-base-300 rounded-box border border-base-300">
      <li v-for="row in rows" :key="row.id" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2 text-ui">
        <code class="font-mono text-xs">••••{{ row.hint }}</code>
        <StatusLabel :tone="row.old ? 'warning' : 'success'" :label="row.old ? labels.old.replace('{date}', row.expires) : labels.current" />
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
          <Icon v-else :name="rows.length === 0 ? 'lucide:key-round' : 'lucide:refresh-cw'" size="16" class="shrink-0" aria-hidden="true" />
          {{ rows.length === 0 ? labels.generate : labels.rotate }}
        </button>
      </div>
      <ErrorText :error="error" />
      <p class="text-xs text-base-content/70">
        {{ labels.personOnly }}
      </p>
    </template>
  </Section>
</template>
