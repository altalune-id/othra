<script setup lang="ts">
import type { SessionView } from '../../lib/account/views';
import { useDates } from '../../composables/useDates';
import { deviceName } from '../../lib/account/views';

const { session } = defineProps<{ session: SessionView }>();

const { t } = useI18n();
const { when } = useDates();
</script>

<template>
  <li class="list-row items-center">
    <Icon name="lucide:monitor-smartphone" size="20" />
    <div class="min-w-0">
      <p class="flex flex-wrap items-center gap-2 font-medium" :title="session.userAgent">
        {{ deviceName(session, t) }}
        <span v-if="session.current" class="badge badge-soft badge-sm">{{ t('sessions.this_device') }}</span>
      </p>
      <p class="text-xs text-base-content/70">
        {{ session.google ? t('methods.google') : t('methods.email') }} · {{ t('sessions.signed_in', { when: when(session.signedInAt) }) }} · {{ t('sessions.active', { when: when(session.lastSeenAt) }) }}
      </p>
    </div>
    <slot />
  </li>
</template>
