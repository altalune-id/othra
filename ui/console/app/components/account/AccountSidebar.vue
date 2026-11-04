<script setup lang="ts">
import SidebarNav from '#base/components/ui/SidebarNav.vue';
import SidebarUser from '~/components/layout/SidebarUser.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { accountBackLink } from './backLink';
import { useAccountNav } from './useAccountNav';

const { t } = useI18n();
const auth = useAuthStore();
const orgs = useOrgsStore();
const back = computed(() => accountBackLink(auth.me?.activeOrgSlug, orgs.items));

const accountNav = useAccountNav();
</script>

<template>
  <SidebarNav :label="t('account.title')" :groups="[accountNav]">
    <template #back>
      <NuxtLink :to="back.to" class="inline-flex items-center gap-1 text-xs text-base-content/70 transition-colors hover:text-base-content">
        <Icon name="lucide:arrow-left" size="14" />
        {{ back.org ? t('account.back_to_org', { org: back.org }) : t('account.back_to_orgs') }}
      </NuxtLink>
    </template>
    <template #footer>
      <SidebarUser />
    </template>
  </SidebarNav>
</template>
