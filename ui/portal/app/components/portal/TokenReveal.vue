<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import SecretField from '#base/components/ui/SecretField.vue';

const { secret, snippet = '' } = defineProps<{ secret: string, snippet?: string }>();

const emit = defineEmits<{ done: [] }>();
const { t } = useI18n();
</script>

<template>
  <section class="flex flex-col gap-3 rounded-box border border-base-300 bg-base-100 p-4" data-testid="reveal">
    <p class="flex items-start gap-2 text-ui">
      <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
      {{ t('tokens.reveal_warning') }}
    </p>
    <SecretField :value="secret" :label="t('common.copy')" />
    <div v-if="snippet" class="flex flex-col gap-1">
      <p class="flex items-center justify-between gap-2 text-xs font-medium text-base-content/70">
        {{ t('tokens.snippet') }}
        <CopyButton :value="snippet" :label="t('tokens.copy_snippet')" />
      </p>
      <pre class="overflow-x-auto rounded-box bg-base-200 p-3 font-mono text-xs"><code>{{ snippet }}</code></pre>
    </div>
    <div>
      <AppButton size="sm" @click="emit('done')">
        {{ t('common.done') }}
      </AppButton>
    </div>
  </section>
</template>
