<script setup lang="ts">
import type { TokenView } from '../../lib/account/views';
import { computed } from 'vue';
import { useDates } from '../../composables/useDates';
import { tokenWhen } from '../../lib/account/views';

const { token } = defineProps<{ token: TokenView }>();

const { t } = useI18n();
const { date, when } = useDates();
const dateLine = computed(() => {
  const w = tokenWhen(token);
  return t(w.key, { date: date(w.date), days: w.days });
});
</script>

<template>
  <li class="list-row items-start">
    <Icon name="lucide:key-round" size="20" class="mt-0.5" />
    <div class="min-w-0 space-y-1">
      <p class="flex flex-wrap items-center gap-2 font-medium">
        <span class="break-all">{{ token.name }}</span>
        <span class="font-mono text-xs text-base-content/70">…{{ token.hint }}</span>
        <span v-if="token.status !== 'active'" class="badge badge-soft badge-sm">{{ t(`tokens.status_${token.status}`) }}</span>
      </p>
      <p class="text-sm">
        {{ token.server }}
      </p>
      <p class="flex flex-wrap gap-1">
        <span v-for="s in token.scopes" :key="s" class="badge badge-outline badge-sm font-mono">{{ s }}</span>
      </p>
      <p class="text-xs text-base-content/70">
        {{ token.lastUsedAt ? t('tokens.last_used', { when: when(token.lastUsedAt) }) : t('tokens.never_used') }}
        ·
        {{ dateLine }}
      </p>
    </div>
    <slot />
  </li>
</template>
