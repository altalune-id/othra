<script setup lang="ts">
import CopyButton from '#base/components/ui/CopyButton.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SystemBadge from '#base/components/ui/SystemBadge.vue';
import IssuerStatus from '~/components/project/overview/IssuerStatus.vue';
import { setupItems } from '~/components/project/overview/setup';
import SetupChecklist from '~/components/project/overview/SetupChecklist.vue';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const { $clientClient: clientApi, $upstreamClient: upstreamApi } = useNuxtApp();

const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() =>
  projects.items.find(item => item.orgId === org.value?.id && item.slug === route.params.project),
);

const guard = useProjectGuard(() => project.value);

if (!project.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const projectId = project.value.id;
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}`);
const detail = useProjectDetailStore();

async function count<T>(load: () => Promise<T[]>): Promise<number> {
  try {
    return (await load()).length;
  } catch {
    return 0;
  }
}

const [clients, upstreams] = await Promise.all([
  count(async () => (await clientApi.listClients({ projectId })).clients),
  count(async () => (await upstreamApi.listUpstreams({ projectId })).upstreams),
  detail.ensureLoaded(projectId),
  useAuthStore().load(),
]);

const full = computed(() => detail.get(projectId));
const methods = computed(() => [
  ...(full.value?.signinEmailLink ? [t('overview.method_email')] : []),
  ...(upstreams ? [t('overview.method_google')] : []),
]);
const setup = computed(() => setupItems({
  issuerHandle: full.value?.issuerHandle ?? '',
  signinEmailLink: !!full.value?.signinEmailLink,
  upstreams,
  clients,
  logoUrl: full.value?.logoUrl ?? '',
  primaryColor: full.value?.primaryColor ?? '',
  termsUrl: full.value?.termsUrl ?? '',
  privacyUrl: full.value?.privacyUrl ?? '',
}, base.value));

useHead({ title: () => `${project.value?.name ?? t('project.title')} · Othra` });
</script>

<template>
  <div v-if="project" class="max-w-4xl space-y-6">
    <PageHeader
      :title="full?.issuerUrl || project.name"
      :mono="!!full?.issuerUrl"
      :description="t('overview.issuer_for', { project: project.name })"
    >
      <template #meta>
        <div class="flex flex-wrap items-center gap-2 text-xs text-base-content/70">
          <img v-if="full?.logoUrl" :src="full.logoUrl" alt="" class="size-10 shrink-0 rounded-box border border-base-300 bg-base-100 object-contain p-1 max-sm:size-8">
          <SystemBadge v-if="project.system" :label="t('common.system')" />
          <span class="font-mono">{{ project.id }}</span>
          <CopyButton :value="project.id" :label="t('common.copy_label', { label: t('common.id') })" />
        </div>
      </template>
      <template v-if="full?.issuerUrl" #actions>
        <CopyButton :value="full.issuerUrl" :label="t('common.copy_label', { label: t('issuer.url') })" />
        <NuxtLink :to="`${base}/settings/issuer`" class="btn btn-ghost btn-sm">
          {{ t('issuer.edit') }}
        </NuxtLink>
      </template>
    </PageHeader>

    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('project.system_note')" />
    <ProjectReadOnlyNote v-else-if="guard.system" />

    <IssuerStatus v-if="full?.issuerUrl" :issuer="full.issuerUrl" :methods="methods" />
    <SetupChecklist v-if="!project.system" :items="setup" />
  </div>
</template>
