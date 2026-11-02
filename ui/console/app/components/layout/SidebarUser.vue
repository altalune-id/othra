<script setup lang="ts">
import ThemeSwitcher from '#base/components/layout/ThemeSwitcher.vue';
import ThemeFamilySwitcher from '~/components/layout/ThemeFamilySwitcher.vue';
import UserMenu from '~/components/layout/UserMenu.vue';
import { useUserMenu } from '~/components/layout/useUserMenu';

const { me, logoutUrl, labels, version, locale, languages, setLocale } = useUserMenu();
</script>

<template>
  <div class="px-1 md:hidden">
    <div class="flex items-center gap-1">
      <ThemeFamilySwitcher />
      <ThemeSwitcher />
    </div>
  </div>
  <UserMenu :me="me" :logout-url="logoutUrl" :labels="labels" :version="version" :locale="locale" :locales="languages" placement="top" @locale="setLocale" />
</template>
