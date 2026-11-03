<script setup lang="ts">
import type { DiscoveryState } from './useDiscoveryCheck';
import KeyValue from '#base/components/ui/KeyValue.vue';
import Section from '#base/components/ui/Section.vue';
import StatusLabel from '#base/components/ui/StatusLabel.vue';

export interface ActiveKey {
  kid: string;
  alg: string;
  days: number;
  next: string;
}

const { state, keys, activeKey, methods, methodsFailed = false, hostedUrl } = defineProps<{
  state: DiscoveryState,
  keys: number | null,
  activeKey: ActiveKey | null,
  methods: string[] | null,
  methodsFailed?: boolean,
  hostedUrl: string,
}>();

const emit = defineEmits<{ retry: [] }>();
const { t } = useI18n();
const rows = computed(() => [
  { label: t('overview.discovery') },
  { label: t('overview.keys') },
  ...(activeKey ? [{ label: t('overview.active_key') }, { label: t('overview.next_rotation') }] : []),
  { label: t('overview.methods') },
]);
const methodsSlot = computed(() => `value-${rows.value.length - 1}`);
const methodsText = computed(() => (methodsFailed ? t('overview.methods_failed') : methods === null ? t('common.loading') : methods.length ? methods.join(', ') : t('overview.methods_none')));
</script>

<template>
  <Section :title="t('overview.status_title')" flush data-testid="issuer-status">
    <KeyValue :items="rows">
      <template #value-0>
        <span v-if="state === 'checking'" role="status" class="inline-flex items-center gap-2">
          <span class="loading loading-spinner loading-xs" aria-hidden="true" />
          {{ t('overview.discovery_checking') }}
        </span>
        <StatusLabel v-else-if="state === 'serving'" tone="success" :label="t('overview.discovery_serving')">
          <span class="font-mono text-xs">/.well-known/openid-configuration</span> {{ t('overview.discovery_checked') }}
        </StatusLabel>
        <span v-else role="status" class="inline-flex flex-wrap items-center gap-2">
          <StatusLabel tone="error" :label="t('overview.discovery_unreachable')" />
          <button type="button" class="btn btn-ghost btn-xs text-ui" @click="emit('retry')">
            <Icon name="lucide:rotate-ccw" size="14" class="shrink-0" aria-hidden="true" />
            {{ t('common.retry') }}
          </button>
        </span>
      </template>
      <template #value-1>
        <StatusLabel v-if="keys !== null" :tone="keys > 0 ? 'success' : 'warning'" :label="t('overview.keys_published', { n: keys })" :detail="keys > 0 ? t('overview.keys_reachable') : ''" />
        <span v-else-if="state === 'checking'" class="text-base-content/70">{{ t('common.loading') }}</span>
        <span v-else class="text-base-content/70">—</span>
      </template>
      <template v-if="activeKey" #value-2>
        <StatusLabel tone="success" mono :label="activeKey.kid" :detail="t('overview.active_key_detail', { alg: activeKey.alg, n: activeKey.days }, activeKey.days)" />
      </template>
      <template v-if="activeKey" #value-3>
        <span :class="{ 'text-base-content/70': !activeKey.next }">{{ activeKey.next || t('overview.next_rotation_none') }}</span>
      </template>
      <template #[methodsSlot]>
        <StatusLabel :tone="methodsFailed ? 'error' : methods?.length ? 'success' : 'warning'" :label="methodsText">
          <a class="link" :href="hostedUrl" target="_blank" rel="noopener">{{ t('overview.hosted_page') }}</a>
        </StatusLabel>
      </template>
    </KeyValue>
  </Section>
</template>
