<script setup lang="ts">
import type { ResourceServerRow } from '~/components/resourceserver/ResourceServerList.vue';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import ErrorText from '#base/components/ui/ErrorText.vue';
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
  <div class="max-w-4xl space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold" tabindex="-1">
          {{ t('rs.title') }}
        </h1>
        <p class="text-sm text-base-content/60">
          {{ t('rs.subtitle') }}
        </p>
      </div>
      <NuxtLink v-if="canEdit" :to="`${base}/new`" class="btn btn-primary btn-sm max-sm:btn-block">
        <Icon name="lucide:plus" size="16" />
        {{ t('rs.new') }}
      </NuxtLink>
    </div>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('rs.system_empty')" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <section class="card border border-base-300 bg-base-100" data-testid="resource-servers-card">
      <div class="card-body gap-2">
        <template v-if="loadError">
          <ErrorText :error="loadError" />
          <div>
            <button type="button" class="btn btn-sm" @click="list.load()">
              {{ t('common.retry') }}
            </button>
          </div>
        </template>
        <div v-else-if="!loading && rows.length === 0" class="flex flex-col items-center gap-2 py-6 text-center">
          <Icon name="lucide:server" size="28" class="text-base-content/60" />
          <p class="font-medium">
            {{ t('rs.empty_title') }}
          </p>
          <p class="max-w-md text-sm text-base-content/70">
            {{ t('rs.empty_body') }}
          </p>
          <NuxtLink v-if="canEdit" :to="`${base}/new`" class="btn btn-primary btn-sm mt-2">
            <Icon name="lucide:plus" size="16" />
            {{ t('rs.new') }}
          </NuxtLink>
        </div>
        <ResourceServerList v-else :rows="rows" :loading="loading" :labels="labels" />
      </div>
    </section>
  </div>
</template>
