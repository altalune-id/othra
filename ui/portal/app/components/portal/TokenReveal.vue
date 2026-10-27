<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import SecretField from '#base/components/ui/SecretField.vue';

const { secret, snippet = '' } = defineProps<{ secret: string, snippet?: string }>();

const emit = defineEmits<{ done: [] }>();
const { t } = useI18n();
</script>

<template>
  <section class="card border border-base-300 bg-base-100" data-testid="reveal">
    <div class="card-body gap-3">
      <p class="flex items-start gap-1 text-sm">
        <Icon name="lucide:triangle-alert" size="14" class="mt-0.5 shrink-0 text-warning" />
        {{ t('tokens.reveal_warning') }}
      </p>
      <SecretField :value="secret" :label="t('common.copy')" />
      <div v-if="snippet" class="space-y-1">
        <p class="flex items-center justify-between gap-2 text-xs font-medium text-base-content/70">
          {{ t('tokens.snippet') }}
          <CopyButton :value="snippet" :label="t('tokens.copy_snippet')" />
        </p>
        <pre class="overflow-x-auto rounded-box bg-base-200 p-3 text-xs"><code>{{ snippet }}</code></pre>
      </div>
      <div class="card-actions justify-end">
        <AppButton size="sm" @click="emit('done')">
          {{ t('common.done') }}
        </AppButton>
      </div>
    </div>
  </section>
</template>
