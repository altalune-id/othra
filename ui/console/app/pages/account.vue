<script setup lang="ts">
import AppShell from '#base/components/ui/AppShell.vue';
import AccountSidebar from '~/components/account/AccountSidebar.vue';
import ConsoleTopBar from '~/components/layout/ConsoleTopBar.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useUIStore } from '~/stores/ui';

const route = useRoute();
const { t } = useI18n();
const ui = useUIStore();
const auth = useAuthStore();
const orgs = useOrgsStore();

watch(() => route.fullPath, () => {
  ui.navOpen = false;
});

await Promise.all([auth.load(), orgs.ensureLoaded()]);
</script>

<template>
  <AppShell v-model:open="ui.navOpen" :close-label="t('nav.close_menu')" full breakpoint="lg">
    <ConsoleTopBar />
    <main class="min-w-0 flex-1 p-4 lg:p-6">
      <NuxtPage />
    </main>
    <template #sidebar>
      <AccountSidebar />
    </template>
  </AppShell>
</template>
