<script setup lang="ts">
import type { MethodRow } from '../../lib/signin/methods';
import { computed } from 'vue';
import { useDates } from '../../composables/useDates';
import { methodEmail } from '../../lib/signin/methods';
import GoogleLogo from '../ui/GoogleLogo.vue';

const { row, accountEmail } = defineProps<{ row: MethodRow, accountEmail: string }>();

const { t } = useI18n();
const { date } = useDates();
const email = computed(() => (row.linked ? methodEmail(row, accountEmail) : ''));
</script>

<template>
  <li class="list-row items-center">
    <GoogleLogo v-if="row.key === 'google'" />
    <Icon v-else name="lucide:mail" size="20" />
    <div class="min-w-0">
      <p class="font-medium">
        {{ t(row.labelKey) }}
      </p>
      <p v-if="email" class="text-sm break-all text-base-content/70">
        {{ email }}
      </p>
      <p v-if="row.key === 'google'" class="text-xs text-base-content/70">
        {{ row.linked ? t('methods.connected_on', { date: date(row.linkedAt) }) : t('methods.not_connected') }}
      </p>
      <p v-if="row.lockKey" class="mt-1 flex items-start gap-1 text-xs text-base-content">
        <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
        {{ t(row.lockKey) }}
      </p>
    </div>
    <slot />
  </li>
</template>
