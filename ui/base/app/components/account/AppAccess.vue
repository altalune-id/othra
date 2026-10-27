<script setup lang="ts">
import type { AppView } from '../../lib/account/views';
import { useDates } from '../../composables/useDates';

const { app, signsIn } = defineProps<{ app: AppView, signsIn: string }>();

const { t } = useI18n();
const { date, when } = useDates();
</script>

<template>
  <div class="min-w-0 space-y-3">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="font-semibold break-words">
          {{ app.name }}
        </p>
        <p v-if="app.host" class="text-xs text-base-content/70">
          {{ t('apps.from_host', { host: app.host }) }}
        </p>
      </div>
      <slot />
    </div>
    <div>
      <p class="text-xs font-medium text-base-content/70">
        {{ t('apps.access') }}
      </p>
      <ul class="mt-1 space-y-2 text-sm">
        <li v-if="app.signsIn">
          {{ signsIn }}
        </li>
        <li v-for="line in app.access" :key="line.server">
          <span class="font-medium">{{ line.server }}</span>
          <ul class="ml-4 list-disc">
            <li v-for="item in line.items" :key="item">
              {{ item }}
            </li>
          </ul>
        </li>
      </ul>
    </div>
    <p class="text-xs text-base-content/70">
      {{ t('apps.connected', { date: date(app.connectedAt) }) }}<template v-if="app.lastUsedAt">
        · {{ t('apps.last_used', { when: when(app.lastUsedAt) }) }}
      </template>
    </p>
  </div>
</template>
