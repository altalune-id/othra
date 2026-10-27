<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import UrlField from '#base/components/ui/UrlField.vue';

const { accept, canEdit, lockReason = '', busy = false, error = null, endpoint, copyLabel } = defineProps<{
  accept: boolean,
  canEdit: boolean,
  lockReason?: '' | 'unverified' | 'no_secret',
  busy?: boolean,
  error?: ShownError | null,
  endpoint: string,
  copyLabel: string,
}>();

const emit = defineEmits<{ set: [accept: boolean] }>();
const { t } = useI18n();
const toggleId = useId();

function onClick(event: MouseEvent) {
  if (lockReason) {
    event.preventDefault();
  }
}

function onChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const wanted = input.checked;
  input.checked = accept;
  emit('set', wanted);
}
</script>

<template>
  <section class="card border border-base-300 bg-base-100" data-testid="pat-card">
    <div class="card-body gap-3">
      <h2 class="card-title text-base">
        {{ t('rs.pat_title') }}
      </h2>
      <p class="text-sm text-base-content/70">
        {{ t('rs.pat_intro') }}
      </p>
      <div v-if="canEdit" class="space-y-1">
        <label :for="toggleId" class="flex items-center gap-3 text-sm font-medium">
          <input
            :id="toggleId"
            :checked="accept"
            type="checkbox"
            class="toggle toggle-sm"
            :class="{ 'cursor-not-allowed': !!lockReason }"
            :disabled="busy"
            :aria-disabled="!!lockReason"
            @click="onClick"
            @change="onChange"
          >
          {{ t('rs.pat_accept') }}
        </label>
        <p v-if="lockReason" class="flex items-start gap-1 text-xs text-base-content">
          <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
          {{ lockReason === 'unverified' ? t('rs.pat_locked_unverified') : t('apperror.RSV012') }}
        </p>
        <ErrorText :error="error" />
      </div>
      <p v-else class="text-sm">
        {{ accept ? t('rs.pat_on') : t('rs.pat_off') }}
      </p>
      <UrlField :label="t('rs.pat_endpoint')" :value="endpoint" size="sm" :copy-label="copyLabel" testid="introspection-endpoint" />
    </div>
  </section>
</template>
