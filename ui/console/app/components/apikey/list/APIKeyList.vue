<script setup lang="ts">
import type { Column, StatusTone } from '#base/components/ui/types';
import type { APIKey } from '~/stores/apiKeys';
import AppBadge from '#base/components/ui/AppBadge.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import AppDropdown from '#base/components/ui/AppDropdown.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import DataTable from '#base/components/ui/DataTable.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import StatusLabel from '#base/components/ui/StatusLabel.vue';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useScopeLabel } from '#base/composables/useScopeLabel';
import { expiryState, keyStatus } from '#base/lib/tokens/keyStatus';
import { useAPIKeysStore } from '~/stores/apiKeys';

const { orgId, canManage = false } = defineProps<{ orgId: string, canManage?: boolean }>();

const { t } = useI18n();
const errorMessage = useErrorMessage();
const scopeLabel = useScopeLabel();
const keys = useAPIKeysStore();
const { $apiKeyClient: client } = useNuxtApp();

await keys.ensureLoaded(orgId);

const statusTone: Record<string, StatusTone> = { active: 'success', expired: 'error', revoked: 'neutral' };
const rows = computed(() => {
  const now = new Date();
  return keys.shared.map(k => ({ ...k, status: keyStatus(!!k.revokedAt, expiryState(k.expiresAt, now)) }));
});

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name') },
  { key: 'kind', label: t('apikeys.col_kind') },
  { key: 'scopes', label: t('apikeys.col_scopes') },
  { key: 'secretHint', label: t('apikeys.col_hint'), mono: true },
  { key: 'status', label: t('apikeys.col_status') },
  ...(canManage ? [{ key: 'menu', label: t('apikeys.col_actions'), hideLabel: true, align: 'end' as const }] : []),
]);

function kindLabel(kind: string): string {
  return kind === 'org' ? t('apikeys.kind_org') : t('apikeys.kind_project');
}

const revoking = ref<APIKey | null>(null);
const busy = ref(false);

function askRevoke(key: APIKey) {
  revoking.value = key;
}

async function confirmRevoke() {
  const key = revoking.value;
  if (!key || busy.value) {
    return;
  }
  busy.value = true;
  try {
    await client.revokeAPIKey({ orgId, hint: key.secretHint });
    keys.markRevoked(key.secretHint);
  } catch (e) {
    keys.error = errorMessage(e, 'error.revoke_apikey');
  } finally {
    busy.value = false;
    revoking.value = null;
  }
}
</script>

<template>
  <LoadError v-if="keys.error" :error="keys.error" :busy="keys.loading" :retry-label="t('common.retry')" @retry="keys.load(orgId)" />
  <EmptyState v-else-if="keys.shared.length === 0" icon="lucide:key-round" :title="t('apikeys.empty')" />
  <DataTable v-else :caption="t('apikeys.title')" row-key="rowKey" :columns="columns" :rows="rows">
    <template #cell-name="{ row }">
      <span class="font-medium">{{ row.name }}</span>
    </template>
    <template #cell-kind="{ row }">
      <AppBadge :label="kindLabel(row.kind)" />
    </template>
    <template #cell-scopes="{ row }">
      <span class="text-base-content/70">{{ row.scopes.map(scopeLabel).join(', ') }}</span>
    </template>
    <template #cell-secretHint="{ row }">
      <span class="text-base-content/70">…{{ row.secretHint }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusLabel :tone="statusTone[row.status] ?? 'neutral'" :label="t(`tokens.status_${row.status}`)" />
    </template>
    <template #cell-menu="{ row }">
      <AppDropdown v-if="row.status === 'active'" placement="end" size="xs" trigger-class="btn-square" content-class="w-44" :label="t('apikeys.actions_for', { name: row.name })">
        <template #trigger>
          <Icon name="lucide:ellipsis" size="14" class="shrink-0" aria-hidden="true" />
        </template>
        <ul class="menu menu-sm w-full p-0 text-ui">
          <li>
            <button type="button" @click="askRevoke(row)">
              <Icon name="lucide:ban" size="16" class="shrink-0 text-error" aria-hidden="true" />
              {{ t('common.revoke') }}
            </button>
          </li>
        </ul>
      </AppDropdown>
    </template>
  </DataTable>

  <AppModal :open="!!revoking" :title="t('apikeys.revoke_confirm')" @close="revoking = null">
    <p class="text-ui">
      {{ t('apikeys.revoke_body', { name: revoking?.name ?? '', hint: revoking?.secretHint ?? '' }) }}
    </p>
    <template #footer>
      <AppButton variant="ghost" size="sm" :disabled="busy" @click="revoking = null">
        {{ t('common.cancel') }}
      </AppButton>
      <AppButton variant="error" size="sm" :disabled="busy" @click="confirmRevoke()">
        {{ t('common.revoke') }}
      </AppButton>
    </template>
  </AppModal>
</template>
