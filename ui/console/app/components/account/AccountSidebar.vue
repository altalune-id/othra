<script setup lang="ts">
import type { NavItem } from '#base/components/ui/types';
import SidebarNav from '#base/components/ui/SidebarNav.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { accountBackLink } from './backLink';

const route = useRoute();
const { t } = useI18n();
const { appVersion } = useRuntimeConfig().public;
const auth = useAuthStore();
const orgs = useOrgsStore();
const back = computed(() => accountBackLink(auth.me?.activeOrgSlug, orgs.items));

const items = computed<NavItem[]>(() => [
  { key: 'profile', label: t('profile.title'), icon: 'lucide:user-round', active: route.path === '/account' },
  { key: 'api-keys', label: t('apikeys.personal_title'), icon: 'lucide:key-round', active: route.path.startsWith('/account/api-keys') },
]);

function onSelect(key: string) {
  return navigateTo(key === 'profile' ? '/account' : `/account/${key}`);
}
</script>

<template>
  <SidebarNav :label="t('account.title')" :items="items" :version="appVersion" @select="onSelect">
    <template #back>
      <NuxtLink :to="back.to" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
        <Icon name="lucide:arrow-left" size="14" />
        {{ back.org ? t('account.back_to_org', { org: back.org }) : t('account.back_to_orgs') }}
      </NuxtLink>
    </template>
  </SidebarNav>
</template>
