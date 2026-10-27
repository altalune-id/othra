<script setup lang="ts">
import CopyButton from '#base/components/ui/CopyButton.vue';
import UrlField from '#base/components/ui/UrlField.vue';
import { discoveryUrls } from '~/components/project/issuer/discovery';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() =>
  projects.items.find(item => item.orgId === org.value?.id && item.slug === route.params.project),
);

if (!project.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}`);
const detail = useProjectDetailStore();
await detail.ensureLoaded(project.value.id);
const full = computed(() => detail.get(project.value?.id ?? ''));
const discovery = computed(() => discoveryUrls(full.value?.issuerUrl ?? ''));
const copyLabel = computed(() => t('common.copy_label', { label: '{label}' }));

const sections = computed(() => [
  { key: 'clients', to: `${base.value}/clients`, icon: 'lucide:app-window', title: t('nav.clients'), hint: t('clients.subtitle') },
  { key: 'resource-servers', to: `${base.value}/resource-servers`, icon: 'lucide:server', title: t('nav.resource_servers'), hint: t('rs.subtitle') },
  { key: 'webhooks', to: `${base.value}/webhooks`, icon: 'lucide:webhook', title: t('webhooks.title'), hint: t('webhooks.subtitle') },
  { key: 'settings', to: `${base.value}/settings`, icon: 'lucide:settings', title: t('nav.settings'), hint: t('project.settings_hint') },
]);

useHead({ title: () => `${project.value?.name ?? t('project.title')} · Othra` });
</script>

<template>
  <div v-if="project" class="space-y-6">
    <div class="flex items-center gap-3">
      <img v-if="full?.logoUrl" :src="full.logoUrl" alt="" class="size-10 shrink-0 rounded-box border border-base-300 bg-base-100 object-contain p-1 max-sm:size-8">
      <div>
        <h1 class="text-2xl font-bold">
          {{ project.name }}
        </h1>
        <p class="text-sm text-base-content/60">
          {{ t('nav.overview') }}
        </p>
        <div class="mt-1 flex items-center gap-1 text-xs text-base-content/60">
          <span class="font-mono">{{ project.id }}</span>
          <CopyButton :value="project.id" :label="t('common.copy')" />
        </div>
      </div>
    </div>

    <section v-if="full" class="card border border-base-300 bg-base-100" data-testid="issuer-card">
      <div class="card-body gap-2">
        <div class="flex items-center justify-between gap-2">
          <h2 class="card-title text-base">
            <Icon name="lucide:fingerprint" size="18" />
            {{ t('overview.issuer_card') }}
          </h2>
          <NuxtLink :to="`${base}/settings/issuer`" class="link text-sm">
            {{ t('issuer.edit') }}
          </NuxtLink>
        </div>
        <UrlField :label="t('issuer.url')" :value="full.issuerUrl" size="sm" :copy-label="copyLabel" testid="overview-issuer-url" />
        <UrlField v-if="discovery" :label="t('overview.discovery')" :value="discovery.oidc" size="sm" :copy-label="copyLabel" testid="overview-discovery-url" />
        <p v-if="full.signinEmailLink" class="text-sm text-base-content/60">
          {{ t('overview.sign_in_line') }}
        </p>
      </div>
    </section>

    <div class="grid gap-3 sm:grid-cols-2">
      <NuxtLink
        v-for="section in sections"
        :key="section.key"
        :to="section.to"
        class="card border border-base-300 bg-base-100 transition-colors hover:border-primary"
      >
        <div class="card-body">
          <h2 class="card-title text-base">
            <Icon :name="section.icon" size="18" />
            {{ section.title }}
          </h2>
          <p class="text-sm text-base-content/60">
            {{ section.hint }}
          </p>
        </div>
      </NuxtLink>
    </div>
  </div>
</template>
