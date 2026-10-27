<script setup lang="ts">
import type { NavItem } from '#base/components/ui/types';
import SidebarNav from '#base/components/ui/SidebarNav.vue';

const route = useRoute();
const { t } = useI18n();
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/settings`);

const items = computed<NavItem[]>(() => [
  { key: 'general', label: t('project.settings_general'), icon: 'lucide:sliders-horizontal', active: route.path === base.value },
  { key: 'issuer', label: t('issuer.title'), icon: 'lucide:fingerprint', active: route.path.startsWith(`${base.value}/issuer`) },
  { key: 'sign-in', label: t('signin.title'), icon: 'lucide:log-in', active: route.path.startsWith(`${base.value}/sign-in`) },
  { key: 'keys', label: t('keys.title'), icon: 'lucide:key-square', active: route.path.startsWith(`${base.value}/keys`) },
  { key: 'branding', label: t('branding.title'), icon: 'lucide:palette', active: route.path.startsWith(`${base.value}/branding`) },
  { key: 'legal', label: t('legal.title'), icon: 'lucide:scale', active: route.path.startsWith(`${base.value}/legal`) },
]);

function onSelect(key: string) {
  return navigateTo(key === 'general' ? base.value : `${base.value}/${key}`);
}
</script>

<template>
  <SidebarNav :label="t('settings.title')" :items="items" @select="onSelect" />
</template>
