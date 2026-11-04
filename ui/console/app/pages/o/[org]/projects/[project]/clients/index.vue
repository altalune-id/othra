<script setup lang="ts">
import type { ClientRow } from '~/components/client/ClientList.vue';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import AppButton from '#base/components/ui/AppButton.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { relativeTime } from '#base/lib/tokens/keyStatus';
import ClientList from '~/components/client/ClientList.vue';
import { clientKinds, isCimdClient, isSystemClient, kindName } from '~/components/client/kinds';
import { redirectHost } from '~/components/client/redirectUris';
import { useClientList } from '~/components/client/useClients';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t, locale } = useI18n();
const now = new Date();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
const guard = useProjectGuard(() => listed.value);
const canEdit = computed(() => guard.value.canConfigure);
await useAuthStore().load();
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/clients`);
const list = useClientList(listed.value.id);
const { loading, loadError } = list;
const rows = computed<ClientRow[]>(() => {
  return list.clients.value.map(c => ({
    id: c.id,
    name: c.name,
    type: isCimdClient(c) ? t('clients.type_mcp') : t(clientKinds[kindName(c.kind)].typeKey),
    icon: isCimdClient(c) ? 'lucide:plug' : clientKinds[kindName(c.kind)].icon,
    host: redirectHost(c.redirectUris) || t('clients.no_redirect'),
    lastUsed: c.lastUsedAt ? relativeTime(timestampDate(c.lastUsedAt), now, locale.value) : t('clients.never_used'),
    to: `${base.value}/${c.id}`,
    system: isSystemClient(c),
  }));
});
const labels = computed(() => ({
  caption: t('clients.title'),
  name: t('common.name'),
  type: t('clients.col_type'),
  clientId: t('clients.client_id'),
  host: t('clients.col_redirect_host'),
  lastUsed: t('clients.col_last_used'),
  actions: t('clients.col_actions'),
  moreFor: t('clients.more_for', { name: '{name}' }),
  open: t('common.open'),
  copyId: t('clients.copy_client_id'),
  copied: t('common.copied'),
  system: t('common.system'),
}));

onMounted(() => list.load());

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('clients.title')} · Othra` });
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('clients.title')" :description="t('clients.subtitle')">
      <template v-if="canEdit" #actions>
        <AppButton size="sm" :to="`${base}/new`">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('clients.new') }}
        </AppButton>
      </template>
    </PageHeader>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('clients.system_note')" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <LoadError v-if="loadError" :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <EmptyState v-else-if="!loading && rows.length === 0" icon="lucide:app-window" :title="t('clients.empty_title')" :body="t('clients.empty_body')">
      <template v-if="canEdit" #actions>
        <AppButton size="sm" :to="`${base}/new`">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('clients.new') }}
        </AppButton>
      </template>
    </EmptyState>
    <ClientList v-else :rows="rows" :loading="loading" :labels="labels" data-testid="clients-card" />
  </div>
</template>
