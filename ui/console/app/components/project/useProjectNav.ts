import type { NavGroup, NavItem } from '#base/components/ui/types';
import { useCan } from '~/composables/useCan';

// useProjectNav lists a project's pages in their sidebar groups, the one source for the sidebar, the breadcrumbs and the palette.
export function useProjectNav() {
  const route = useRoute();
  const { t } = useI18n();
  const canSeeEndUsers = useCan('end_user:read');
  const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}`);

  function item(key: string, label: string, path: string, icon: string, exact = false): NavItem {
    const to = `${base.value}${path}`;
    return { key, label, to, icon, active: exact ? route.path === to : route.path.startsWith(to) };
  }

  return computed<NavGroup[]>(() => [
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
        item('legal', t('nav.legal'), '/settings/legal', 'lucide:scale'),
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
  ]);
}
