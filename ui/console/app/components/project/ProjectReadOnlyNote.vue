<script setup lang="ts">
const { text = '' } = defineProps<{ text?: string }>();
const { t } = useI18n();
</script>

<template>
  <div role="note" class="alert alert-soft">
    <Icon name="lucide:eye" size="18" class="text-info" />
    <span class="text-sm text-base-content">{{ text || t('project.read_only') }}</span>
  </div>
</template>
