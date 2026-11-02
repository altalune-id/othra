<script setup lang="ts">
import ThemeSwitcher from '#base/components/layout/ThemeSwitcher.vue';
import NavbarShell from '#base/components/ui/NavbarShell.vue';
import ThemeFamilySwitcher from '~/components/layout/ThemeFamilySwitcher.vue';
import UserMenu from '~/components/layout/UserMenu.vue';
import { useUserMenu } from '~/components/layout/useUserMenu';
import { logoURL } from '~/lib/auth/urls';
import { useAuthStore } from '~/stores/auth';

const route = useRoute();

const inAccount = computed(() => route.path.startsWith('/account'));
const hasNav = computed(() => !!route.params.org || inAccount.value);

const auth = useAuthStore();
const { apiUrl } = useRuntimeConfig().public;
const logoUrl = logoURL(apiUrl);
const { me, logoutUrl, labels, locale, languages, setLocale } = useUserMenu();
onMounted(() => auth.load());
</script>

<template>
  <NavbarShell v-if="!hasNav">
    <template #brand>
      <NuxtLink to="/" class="btn btn-ghost gap-2 px-2 text-base font-bold tracking-tight">
        <img :src="logoUrl" alt="" width="28" height="28" class="h-7 w-7 rounded-md object-contain">
        Othra
      </NuxtLink>
    </template>
    <template #end>
      <ThemeFamilySwitcher />
      <ThemeSwitcher />
      <UserMenu :me="me" :logout-url="logoutUrl" :labels="labels" :locale="locale" :locales="languages" @locale="setLocale" />
    </template>
  </NavbarShell>
</template>
