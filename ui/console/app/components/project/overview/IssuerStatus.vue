<script setup lang="ts">
import AppModal from '#base/components/ui/AppModal.vue';
import KeyValue from '#base/components/ui/KeyValue.vue';
import Section from '#base/components/ui/Section.vue';
import { useDiscoveryCheck } from './useDiscoveryCheck';

const { issuer, methods } = defineProps<{ issuer: string, methods: string[] | null }>();

const { t } = useI18n();
const { state, doc, keys, check } = useDiscoveryCheck(() => issuer);
const showJSON = ref(false);
const items = computed(() => [
  { label: t('overview.discovery') },
  { label: t('overview.keys'), value: keys.value === null ? '' : t('overview.keys_published', { n: keys.value }) },
  { label: t('overview.methods'), value: methods === null ? t('common.loading') : methods.length ? methods.join(', ') : t('overview.methods_none') },
]);

onMounted(check);
</script>

<template>
  <Section :title="t('overview.status_title')" :description="t('overview.cache_note')" data-testid="issuer-status">
    <template #actions>
      <button type="button" class="btn btn-sm" :disabled="!doc" @click="showJSON = true">
        <Icon name="lucide:braces" size="16" />
        {{ t('overview.view_json') }}
      </button>
    </template>
    <KeyValue :items="items">
      <template #value-0>
        <span class="flex flex-wrap items-center gap-2" role="status">
          <span v-if="state === 'checking'" class="loading loading-spinner loading-xs" aria-hidden="true" />
          <span v-else class="status" :class="state === 'serving' ? 'status-success' : 'status-error'" aria-hidden="true" />
          {{ t(`overview.discovery_${state}`) }}
          <button v-if="state === 'unreachable'" type="button" class="btn btn-ghost btn-xs" @click="check">
            <Icon name="lucide:rotate-ccw" size="14" />
            {{ t('common.retry') }}
          </button>
        </span>
      </template>
    </KeyValue>
    <AppModal :open="showJSON" :title="t('overview.discovery')" @close="showJSON = false">
      <pre class="max-h-96 overflow-auto rounded-box bg-base-200 p-3 font-mono text-xs" data-testid="discovery-json">{{ JSON.stringify(doc, null, 2) }}</pre>
      <template #footer>
        <button type="button" class="btn btn-sm" @click="showJSON = false">
          {{ t('common.close') }}
        </button>
      </template>
    </AppModal>
  </Section>
</template>
