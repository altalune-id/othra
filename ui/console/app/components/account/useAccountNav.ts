import type { NavGroup } from '#base/components/ui/types';

// useAccountNav lists a person's own account pages, the one source for the account sidebar, the breadcrumbs and the palette.
export function useAccountNav() {
  const route = useRoute();
  const { t } = useI18n();
  return computed<NavGroup>(() => ({
    items: [
      { key: 'profile', label: t('profile.title'), to: '/account', icon: 'lucide:user-round', active: route.path === '/account' },
      { key: 'api-keys', label: t('apikeys.personal_title'), to: '/account/api-keys', icon: 'lucide:key-round', active: route.path.startsWith('/account/api-keys') },
    ],
  }));
}
