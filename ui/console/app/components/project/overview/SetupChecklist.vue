<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import type { SetupItem } from './setup';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import Section from '#base/components/ui/Section.vue';

const { items, loading = false, error = null } = defineProps<{ items: SetupItem[], loading?: boolean, error?: ShownError | null }>();
const emit = defineEmits<{ retry: [] }>();
const ready = computed(() => !loading && !error);

const { t } = useI18n();
const done = computed(() => items.filter(i => i.done).length);
const progress = computed(() => t('overview.setup_progress', { done: done.value, total: items.length }));
</script>

<template>
  <Section :title="t('overview.setup_title')" :description="ready ? progress : ''" flush class="self-start" data-testid="setup-checklist">
    <template v-if="ready" #header>
      <progress class="progress mt-1 h-1" :value="done" :max="items.length" :aria-label="t('overview.setup_title')" :aria-valuetext="progress" />
    </template>
    <div v-if="error" class="p-4">
      <LoadError :error="error" :retry-label="t('common.retry')" @retry="emit('retry')" />
    </div>
    <div v-else-if="loading" class="space-y-3 p-4" data-testid="setup-loading">
      <LoadingStatus :label="t('common.loading')" />
      <div v-for="n in 5" :key="n" class="skeleton h-5 w-full" aria-hidden="true" />
    </div>
    <ul v-else class="divide-y divide-base-300">
      <li v-for="item in items" :key="item.key" class="flex items-start gap-2 px-4 py-2 text-ui">
        <Icon :name="item.done ? 'lucide:circle-check' : 'lucide:circle-dashed'" size="16" class="mt-0.5 shrink-0" aria-hidden="true" />
        <span v-if="item.done" class="min-w-0 flex-1 text-base-content/70"><span class="sr-only">{{ t('overview.setup_done') }}: </span>{{ t(`overview.setup_${item.key}`) }}</span>
        <span v-else class="min-w-0 flex-1 font-medium">{{ t(`overview.setup_${item.key}`) }}</span>
        <NuxtLink v-if="!item.done" :to="item.to" class="link text-ui">
          {{ t('overview.setup_go') }}
        </NuxtLink>
      </li>
    </ul>
  </Section>
</template>
