<script setup lang="ts">
import type { APIKey } from '~/stores/apiKeys';
import AppModal from '~/components/ui/AppModal.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useScopeLabel } from '~/composables/useScopeLabel';
import { useAPIKeysStore } from '~/stores/apiKeys';

const { orgId, canManage = false } = defineProps<{ orgId: string, canManage?: boolean }>();

const { t } = useI18n();
const errorMessage = useErrorMessage();
const scopeLabel = useScopeLabel();
const keys = useAPIKeysStore();
const { $apiKeyClient: client } = useNuxtApp();

await keys.ensureLoaded(orgId);

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
  <ErrorText v-if="keys.error" :error="keys.error" />
  <p v-else-if="keys.shared.length === 0" class="text-sm text-base-content/60">
    {{ t('apikeys.empty') }}
  </p>
  <div v-else class="overflow-x-auto rounded-box border border-base-content/5 bg-base-100">
    <table class="table table-sm">
      <thead>
        <tr>
          <th>{{ t('common.name') }}</th>
          <th>{{ t('apikeys.col_kind') }}</th>
          <th>{{ t('apikeys.col_scopes') }}</th>
          <th>{{ t('apikeys.col_hint') }}</th>
          <th>{{ t('apikeys.col_status') }}</th>
          <th />
        </tr>
      </thead>
      <tbody>
        <tr v-for="k in keys.shared" :key="k.rowKey" class="hover:bg-base-200">
          <td>{{ k.name }}</td>
          <td>{{ kindLabel(k.kind) }}</td>
          <td class="text-base-content/60">
            {{ k.scopes.map(scopeLabel).join(', ') }}
          </td>
          <td class="font-mono text-xs text-base-content/60">
            …{{ k.secretHint }}
          </td>
          <td>
            <span class="badge badge-sm" :class="k.revokedAt ? 'badge-error' : 'badge-success'">
              {{ k.revokedAt ? t('apikeys.revoked') : t('apikeys.active') }}
            </span>
          </td>
          <td class="whitespace-nowrap text-right">
            <button v-if="canManage && !k.revokedAt" type="button" class="btn btn-ghost btn-xs" @click="askRevoke(k)">
              <Icon name="lucide:ban" size="14" class="text-error" />
              {{ t('common.revoke') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <AppModal :open="!!revoking" :title="t('apikeys.revoke_confirm')" @close="revoking = null">
    <p class="text-sm">
      {{ t('apikeys.revoke_body', { name: revoking?.name ?? '', hint: revoking?.secretHint ?? '' }) }}
    </p>
    <template #footer>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="revoking = null">
        {{ t('common.cancel') }}
      </button>
      <button type="button" class="btn btn-error btn-sm" :disabled="busy" @click="confirmRevoke()">
        {{ t('common.revoke') }}
      </button>
    </template>
  </AppModal>
</template>
