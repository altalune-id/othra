<script setup lang="ts">
import { useTheme } from '#base/components/layout/useTheme';

const { init } = useTheme();
const { t, locale } = useI18n();
const { project } = useRuntimeConfig().public;

useHead({
  htmlAttrs: { lang: locale },
  titleTemplate: title => (title ? `${title} · ${project}` : t('portal.title', { project })),
});

onMounted(init);
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
