<script setup lang="ts">
import type { ResourceServerRow } from '~/components/resourceserver/ResourceServerList.vue';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import AppButton from '#base/components/ui/AppButton.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { useDates } from '#base/composables/useDates';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { resourceServerListLabels } from '~/components/resourceserver/labels';
import ResourceServerList from '~/components/resourceserver/ResourceServerList.vue';
import { useResourceServerList } from '~/components/resourceserver/useResourceServers';
import { relativeTime, verificationName, verificationStates } from '~/components/resourceserver/verification';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t, locale } = useI18n();
const { moment } = useDates();
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

const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/resource-servers`);
const list = useResourceServerList(listed.value.id);
const { loading, loadError } = list;
const rows = computed<ResourceServerRow[]>(() => {
  return list.servers.value.map((rs) => {
    const status = verificationName(rs.state, rs.trusted);
    const checked = rs.lastCheckedAt && !rs.trusted ? timestampDate(rs.lastCheckedAt) : null;
    return {
      id: rs.id,
      name: rs.name,
      identifier: rs.identifier,
      status,
      statusLabel: t(verificationStates[status].labelKey),
      checked: checked ? relativeTime(checked, locale.value) : '',
      checkedTitle: moment(checked),
      to: `${base.value}/${rs.id}`,
    };
  });
});
const labels = computed(() => resourceServerListLabels(t));

onMounted(() => list.load());

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('rs.title')} · Othra` });
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('rs.title')" :description="t('rs.subtitle')">
      <template v-if="canEdit" #actions>
        <AppButton size="sm" :to="`${base}/new`">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('rs.new') }}
        </AppButton>
      </template>
    </PageHeader>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('rs.system_empty')" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <LoadError v-if="loadError" :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <EmptyState v-else-if="!loading && rows.length === 0" icon="lucide:server" :title="t('rs.empty_title')" :body="t('rs.empty_body')">
      <template v-if="canEdit" #actions>
        <AppButton size="sm" :to="`${base}/new`">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('rs.new') }}
        </AppButton>
      </template>
    </EmptyState>
    <ResourceServerList v-else :rows="rows" :loading="loading" :labels="labels" data-testid="resource-servers-card" />
  </div>
</template>
