<script setup lang="ts">
import type { Column } from '#base/components/ui/types';
import type { Webhook } from '~/stores/webhooks';
import AppButton from '#base/components/ui/AppButton.vue';
import AppDropdown from '#base/components/ui/AppDropdown.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import DataTable from '#base/components/ui/DataTable.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import SecretField from '#base/components/ui/SecretField.vue';
import StatusLabel from '#base/components/ui/StatusLabel.vue';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { webhookEvents } from '~/components/webhook/events';
import { toWebhook, useWebhooksStore } from '~/stores/webhooks';

const { orgId, projectId } = defineProps<{ orgId: string, projectId: string }>();

const route = useRoute();
const { t } = useI18n();
const errorMessage = useErrorMessage();
const webhooks = useWebhooksStore();
const { $webhookClient: client } = useNuxtApp();

void webhooks.ensureLoaded(orgId, projectId);

const items = computed(() => webhooks.forProject(projectId));
const labelByName = new Map(webhookEvents.map(e => [e.name, e.labelKey]));

const columns = computed<Column[]>(() => [
  { key: 'url', label: t('webhooks.col_url'), mono: true },
  { key: 'events', label: t('webhooks.col_events') },
  { key: 'status', label: t('webhooks.col_status') },
  { key: 'menu', label: t('webhooks.col_actions'), hideLabel: true, align: 'end' },
]);
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/webhooks`);

function events(webhook: Webhook) {
  return webhook.eventTypes.map(n => t(labelByName.get(n) ?? n)).join(', ');
}

const rotating = ref<Webhook | null>(null);
const rotatedSecret = ref('');
const busyId = ref('');

async function ping(webhook: Webhook) {
  busyId.value = webhook.id;
  try {
    await client.pingWebhook({ orgId, projectId, id: webhook.id });
  } catch (e) {
    webhooks.error = errorMessage(e, 'error.ping_webhook');
  } finally {
    busyId.value = '';
  }
}

async function toggle(webhook: Webhook) {
  busyId.value = webhook.id;
  try {
    const res = await client.updateWebhook({
      orgId,
      projectId,
      id: webhook.id,
      url: webhook.url,
      description: webhook.description,
      eventTypes: webhook.eventTypes,
      active: !webhook.active,
    });
    if (res.webhook) {
      webhooks.replace(toWebhook(res.webhook));
    }
  } catch (e) {
    webhooks.error = errorMessage(e, 'error.update_webhook');
  } finally {
    busyId.value = '';
  }
}

async function rotate(webhook: Webhook) {
  busyId.value = webhook.id;
  try {
    const res = await client.rotateWebhookSecret({ orgId, projectId, id: webhook.id });
    rotating.value = webhook;
    rotatedSecret.value = res.secret;
  } catch (e) {
    webhooks.error = errorMessage(e, 'error.rotate_webhook');
  } finally {
    busyId.value = '';
  }
}

const deleting = ref<Webhook | null>(null);

function askDelete(webhook: Webhook) {
  deleting.value = webhook;
}

async function confirmDelete() {
  const webhook = deleting.value;
  if (!webhook || busyId.value) {
    return;
  }
  deleting.value = null;
  busyId.value = webhook.id;
  try {
    await client.deleteWebhook({ orgId, projectId, id: webhook.id });
    webhooks.remove(webhook.id);
  } catch (e) {
    webhooks.error = errorMessage(e, 'error.delete_webhook');
  } finally {
    busyId.value = '';
  }
}

function closeRotate() {
  rotating.value = null;
  rotatedSecret.value = '';
}
</script>

<template>
  <LoadError v-if="webhooks.error" :error="webhooks.error" :busy="webhooks.loading" :retry-label="t('common.retry')" @retry="webhooks.load(orgId, projectId)" />
  <LoadingStatus v-else-if="webhooks.loading && items.length === 0" :label="t('common.loading')" />
  <EmptyState v-else-if="items.length === 0" icon="lucide:webhook" :title="t('webhooks.empty')" />
  <DataTable v-else :caption="t('webhooks.title')" row-key="id" :columns="columns" :rows="items">
    <template #cell-url="{ row }">
      <NuxtLink :to="`${base}/${row.id}`" class="link link-hover block max-w-80 truncate" :title="row.url">
        {{ row.url }}
      </NuxtLink>
    </template>
    <template #cell-events="{ row }">
      <span class="text-base-content/70">{{ events(row) }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusLabel :tone="row.active ? 'success' : 'neutral'" :label="row.active ? t('webhooks.active') : t('webhooks.inactive')" />
    </template>
    <template #cell-menu="{ row }">
      <AppDropdown placement="end" size="xs" trigger-class="btn-square" content-class="w-48" :label="t('webhooks.more_for', { url: row.url })" :disabled="busyId === row.id">
        <template #trigger>
          <Icon name="lucide:ellipsis" size="14" class="shrink-0" aria-hidden="true" />
        </template>
        <ul class="menu menu-sm w-full p-0 text-ui">
          <li>
            <NuxtLink :to="`${base}/${row.id}`">
              <Icon name="lucide:scroll-text" size="16" class="shrink-0" aria-hidden="true" />
              {{ t('webhooks.logs') }}
            </NuxtLink>
          </li>
          <li>
            <button type="button" @click="ping(row)">
              <Icon name="lucide:send" size="16" class="shrink-0" aria-hidden="true" />
              {{ t('webhooks.ping') }}
            </button>
          </li>
          <li>
            <button type="button" @click="toggle(row)">
              <Icon :name="row.active ? 'lucide:pause' : 'lucide:play'" size="16" class="shrink-0" aria-hidden="true" />
              {{ row.active ? t('webhooks.deactivate') : t('webhooks.activate') }}
            </button>
          </li>
          <li>
            <button type="button" @click="rotate(row)">
              <Icon name="lucide:key-round" size="16" class="shrink-0" aria-hidden="true" />
              {{ t('webhooks.rotate') }}
            </button>
          </li>
          <li>
            <button type="button" @click="askDelete(row)">
              <Icon name="lucide:trash-2" size="16" class="shrink-0 text-error" aria-hidden="true" />
              {{ t('common.delete') }}
            </button>
          </li>
        </ul>
      </AppDropdown>
    </template>
  </DataTable>

  <AppModal :open="!!deleting" :title="t('webhooks.delete_title')" @close="deleting = null">
    <p class="text-ui break-words">
      {{ t('webhooks.delete_body', { url: deleting?.url ?? '' }) }}
    </p>
    <template #footer>
      <AppButton variant="ghost" size="sm" autofocus @click="deleting = null">
        {{ t('common.cancel') }}
      </AppButton>
      <AppButton variant="error" size="sm" @click="confirmDelete()">
        {{ t('common.delete') }}
      </AppButton>
    </template>
  </AppModal>

  <AppModal :open="!!rotating" :title="t('webhooks.rotate')" :dismissible="!rotatedSecret" @close="closeRotate()">
    <div class="flex flex-col gap-3">
      <p class="flex items-start gap-2 text-ui">
        <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
        {{ t('webhooks.secret_warning') }}
      </p>
      <SecretField :value="rotatedSecret" :label="t('common.copy')" />
    </div>
    <template #footer>
      <AppButton size="sm" @click="closeRotate()">
        {{ t('common.done') }}
      </AppButton>
    </template>
  </AppModal>
</template>
