<script setup lang="ts">
import type { SetupItem } from './setup';
import Section from '#base/components/ui/Section.vue';

const { items } = defineProps<{ items: SetupItem[] }>();

const { t } = useI18n();
const done = computed(() => items.filter(i => i.done).length);
const progress = computed(() => t('overview.setup_progress', { done: done.value, total: items.length }));
</script>

<template>
  <Section :title="t('overview.setup_title')" :description="progress" data-testid="setup-checklist">
    <progress class="progress progress-primary w-full" :value="done" :max="items.length" :aria-label="t('overview.setup_title')" :aria-valuetext="progress" />
    <ul class="divide-y divide-base-300">
      <li v-for="item in items" :key="item.key" class="flex items-center justify-between gap-3 py-2 text-sm">
        <span class="flex items-center gap-2">
          <Icon :name="item.done ? 'lucide:circle-check' : 'lucide:circle'" size="18" :class="item.done ? 'text-success' : 'text-base-content/40'" />
          {{ t(`overview.setup_${item.key}`) }}
          <span class="sr-only">{{ item.done ? t('overview.setup_done') : t('overview.setup_todo') }}</span>
        </span>
        <NuxtLink v-if="!item.done" :to="item.to" class="btn btn-sm">
          {{ t('overview.setup_go') }}
        </NuxtLink>
      </li>
    </ul>
  </Section>
</template>
