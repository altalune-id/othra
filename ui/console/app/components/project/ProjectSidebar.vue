<script setup lang="ts">
import type { NavGroup, NavItem } from '#base/components/ui/types';
import SidebarNav from '#base/components/ui/SidebarNav.vue';
import OrgRoleBadge from '~/components/org/OrgRoleBadge.vue';
import OrgSelector from '~/components/org/OrgSelector.vue';
import ProjectSelector from '~/components/project/ProjectSelector.vue';
import { useCan } from '~/composables/useCan';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const { appVersion } = useRuntimeConfig().public;
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}`);
const canSeeEndUsers = useCan('end_user:read');

function item(key: string, label: string, path: string, icon: string, exact = false): NavItem {
  const to = `${base.value}${path}`;
  return { key, label, to, icon, active: exact ? route.path === to : route.path.startsWith(to) };
}

const groups = computed<NavGroup[]>(() => [
  { items: [item('overview', t('nav.overview'), '', 'lucide:activity', true)] },
  {
    label: t('nav.group_applications'),
    items: [
      item('clients', t('nav.clients'), '/clients', 'lucide:app-window'),
      item('resource-servers', t('nav.resource_servers'), '/resource-servers', 'lucide:server'),
    ],
  },
  ...(canSeeEndUsers.value ? [{ label: t('nav.group_people'), items: [item('end-users', t('nav.end_users'), '/end-users', 'lucide:user-round')] }] : []),
  {
    label: t('nav.group_signin'),
    items: [
      item('sign-in', t('nav.methods'), '/settings/sign-in', 'lucide:log-in'),
      item('branding', t('branding.title'), '/settings/branding', 'lucide:palette'),
      item('legal', t('legal.title'), '/settings/legal', 'lucide:scale'),
    ],
  },
  {
    label: t('nav.group_security'),
    items: [
      item('keys', t('keys.title'), '/settings/keys', 'lucide:key-square'),
      item('issuer', t('issuer.title'), '/settings/issuer', 'lucide:badge-check'),
    ],
  },
  { label: t('nav.group_developers'), items: [item('webhooks', t('webhooks.title'), '/webhooks', 'lucide:webhook')] },
  { items: [item('settings', t('nav.project_settings'), '/settings', 'lucide:settings', true)] },
]);
</script>

<template>
  <SidebarNav :label="project?.name ?? ''" :groups="groups">
    <template #top>
      <div class="flex flex-col items-start gap-1">
        <div class="flex flex-wrap items-center gap-1">
          <OrgSelector />
          <OrgRoleBadge />
        </div>
        <ProjectSelector />
      </div>
    </template>
    <template v-if="appVersion" #footer>
      <span class="px-2 text-xs text-base-content/70">Othra v{{ appVersion }}</span>
    </template>
  </SidebarNav>
</template>
