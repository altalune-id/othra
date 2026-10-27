<script setup lang="ts">
import AccountSidebar from '~/components/account/AccountSidebar.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useUIStore } from '~/stores/ui';

const route = useRoute();
const ui = useUIStore();
const auth = useAuthStore();
const orgs = useOrgsStore();

watch(() => route.fullPath, () => {
  ui.navOpen = false;
});

await Promise.all([auth.load(), orgs.ensureLoaded()]);
</script>

<template>
  <div class="flex flex-1">
    <div v-if="ui.navOpen" class="fixed inset-x-0 top-14 bottom-0 z-40 bg-black/40 md:hidden" @click="ui.navOpen = false" />
    <div
      class="z-40"
      :class="ui.navOpen
        ? 'fixed top-14 bottom-0 left-0 flex flex-col overflow-y-auto bg-base-100 shadow-xl md:static md:z-auto md:flex-row md:overflow-visible md:bg-transparent md:shadow-none'
        : 'hidden md:flex md:flex-row'"
    >
      <AccountSidebar />
    </div>
    <main class="min-w-0 flex-1 p-4 sm:p-6">
      <NuxtPage />
    </main>
  </div>
</template>
