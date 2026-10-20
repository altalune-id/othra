<script setup lang="ts">
import type { NavItem } from '~/components/ui/types';
import SidebarNav from '~/components/ui/SidebarNav.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const { appVersion } = useRuntimeConfig().public;
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const projectSlug = computed(() => route.params.project as string);
const project = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === projectSlug.value));
const base = computed(() => `/o/${route.params.org}/projects/${projectSlug.value}`);

const items = computed<NavItem[]>(() => [
  { key: 'overview', label: t('nav.overview'), icon: 'lucide:layout-dashboard', active: route.path === base.value },
  { key: 'clients', label: t('nav.clients'), icon: 'lucide:app-window', active: route.path.startsWith(`${base.value}/clients`) },
  { key: 'resource-servers', label: t('nav.resource_servers'), icon: 'lucide:server', active: route.path.startsWith(`${base.value}/resource-servers`) },
  { key: 'webhooks', label: t('webhooks.title'), icon: 'lucide:webhook', active: route.path.startsWith(`${base.value}/webhooks`) },
  { key: 'settings', label: t('nav.settings'), icon: 'lucide:settings', active: route.path.startsWith(`${base.value}/settings`) },
]);

const targets: Record<string, string> = {
  'overview': '',
  'clients': '/clients',
  'resource-servers': '/resource-servers',
  'webhooks': '/webhooks',
  'settings': '/settings',
};

function onSelect(key: string) {
  return navigateTo(`${base.value}${targets[key] ?? ''}`);
}
</script>

<template>
  <SidebarNav :label="project?.name ?? ''" :items="items" :version="appVersion" @select="onSelect">
    <template #back>
      <NuxtLink :to="`/o/${route.params.org}/projects`" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
        <Icon name="lucide:chevron-left" size="14" />
        {{ t('project.all') }}
      </NuxtLink>
    </template>
  </SidebarNav>
</template>
