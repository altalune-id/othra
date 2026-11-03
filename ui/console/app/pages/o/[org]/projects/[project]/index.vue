<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import AppButton from '#base/components/ui/AppButton.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SidePanel from '#base/components/ui/SidePanel.vue';
import SystemBadge from '#base/components/ui/SystemBadge.vue';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { formatDate } from '#base/lib/format/date';
import { SigningKeyStatus } from '#gen/keys/v1/keys_pb';
import { useSigningKeys } from '~/components/project/keys/useSigningKeys';
import IssuerStatus from '~/components/project/overview/IssuerStatus.vue';
import KeyRotation from '~/components/project/overview/KeyRotation.vue';
import { setupItems } from '~/components/project/overview/setup';
import SetupChecklist from '~/components/project/overview/SetupChecklist.vue';
import { useDiscoveryCheck } from '~/components/project/overview/useDiscoveryCheck';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t, locale } = useI18n();
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

const counts = ref<{ clients: number, upstreams: number } | null>(null);
const countsError = ref<ShownError | null>(null);
const errorMessage = useErrorMessage();
await Promise.all([detail.ensureLoaded(projectId), useAuthStore().load()]);

async function loadCounts() {
  counts.value = null;
  countsError.value = null;
  try {
    const [c, u] = await Promise.all([clientApi.listClients({ projectId }), upstreamApi.listUpstreams({ projectId })]);
    counts.value = { clients: c.clients.length, upstreams: u.upstreams.length };
  } catch (e) {
    countsError.value = errorMessage(e, 'overview.counts_failed');
  }
}

const signingKeys = useSigningKeys(projectId);
const issuer = computed(() => detail.get(projectId)?.issuerUrl ?? '');
const discovery = useDiscoveryCheck(issuer);
const showJSON = ref(false);
const hostedUrl = computed(() => (issuer.value ? `${issuer.value.replace(/\/$/, '')}/account/` : ''));

onMounted(() => {
  void loadCounts();
  void signingKeys.load();
  if (issuer.value) {
    void discovery.check();
  }
});
onBeforeUnmount(signingKeys.dispose);

const activeKey = computed(() => {
  const k = signingKeys.keys.value.find(key => key.status === SigningKeyStatus.ACTIVE);
  const since = k?.activatedAt ?? k?.createdAt;
  const next = signingKeys.keys.value.find(key => key.status === SigningKeyStatus.NEXT)?.activatesAt;
  return k ? { kid: k.kid.slice(0, 8), alg: k.alg, days: since ? Math.max(0, Math.floor((Date.now() - timestampDate(since).getTime()) / 86_400_000)) : 0, next: next ? formatDate(timestampDate(next), locale.value) : '' } : null;
});

const full = computed(() => detail.get(projectId));
const setupError = computed(() => countsError.value ?? (full.value ? null : detail.error));

function retrySetup() {
  if (!full.value) {
    void detail.load(projectId);
  }
  if (countsError.value) {
    void loadCounts();
  }
}
const methods = computed(() => (counts.value
  ? [
      ...(full.value?.signinEmailLink ? [t('overview.method_email')] : []),
      ...(counts.value.upstreams ? [t('overview.method_google')] : []),
    ]
  : null));
const setup = computed(() => setupItems({
  issuerHandle: full.value?.issuerHandle ?? '',
  signinEmailLink: !!full.value?.signinEmailLink,
  upstreams: counts.value?.upstreams ?? 0,
  clients: counts.value?.clients ?? 0,
  logoUrl: full.value?.logoUrl ?? '',
  primaryColor: full.value?.primaryColor ?? '',
  termsUrl: full.value?.termsUrl ?? '',
  privacyUrl: full.value?.privacyUrl ?? '',
}, base.value));

useHead({ title: () => `${project.value?.name ?? t('project.title')} · Othra` });
</script>

<template>
  <div v-if="project" class="flex flex-col gap-5">
    <PageHeader
      :title="full?.issuerUrl || project.name"
      :mono="!!full?.issuerUrl"
      :description="t('overview.issuer_for', { project: project.name })"
    >
      <template v-if="project.system" #meta>
        <SystemBadge :label="t('common.system')" />
      </template>
      <template #status>
        <p class="flex items-center gap-1 text-xs text-base-content/70">
          <span class="font-mono">{{ project.id }}</span>
          <CopyButton :value="project.id" :label="t('common.copy_label', { label: t('common.id') })" :copied-label="t('common.copied')" />
        </p>
      </template>
      <template v-if="full?.issuerUrl" #actions>
        <CopyButton :value="full.issuerUrl" :label="t('overview.copy_issuer')" :text="t('overview.copy_issuer')" :copied-label="t('common.copied')" />
        <AppButton variant="plain" size="sm" :disabled="!discovery.doc.value" @click="showJSON = true">
          <Icon name="lucide:braces" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('overview.view_json') }}
        </AppButton>
        <AppButton size="sm" :href="hostedUrl" target="_blank" rel="noopener">
          <Icon name="lucide:play" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('overview.try_signin') }}
        </AppButton>
      </template>
    </PageHeader>

    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('project.system_note')" />
    <ProjectReadOnlyNote v-else-if="guard.system" />

    <div class="grid min-w-0 grid-cols-1 gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(16rem,1fr)]">
      <div class="flex min-w-0 flex-col gap-5">
        <IssuerStatus
          v-if="full?.issuerUrl"
          :state="discovery.state.value"
          :keys="discovery.keys.value"
          :active-key="activeKey"
          :methods="methods"
          :methods-failed="!!countsError"
          :hosted-url="hostedUrl"
          @retry="discovery.check()"
        />
        <p v-if="full?.issuerUrl" class="-mt-3 text-ui text-base-content/70" data-testid="cache-note">
          {{ t('overview.cache_note') }}
        </p>
        <KeyRotation :keys="signingKeys.keys.value" />
      </div>
      <SetupChecklist v-if="!project.system" :items="setup" :loading="!counts && !setupError" :error="setupError" @retry="retrySetup" />
    </div>

    <SidePanel v-model:open="showJSON" :title="t('overview.discovery_doc')" :close-label="t('overview.discovery_doc_close')">
      <p class="text-ui text-base-content/70">
        {{ t('overview.discovery_doc_caption') }}
      </p>
      <pre tabindex="0" :aria-label="t('overview.discovery_doc')" class="overflow-x-auto rounded-box bg-base-200 p-3 font-mono text-xs" data-testid="discovery-json">{{ JSON.stringify(discovery.doc.value, null, 2) }}</pre>
    </SidePanel>
  </div>
</template>
