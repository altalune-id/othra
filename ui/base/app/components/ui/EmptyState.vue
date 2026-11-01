<script setup lang="ts">
const { icon, title, body = '', headingLevel = 2 } = defineProps<{ icon: string, title: string, body?: string, headingLevel?: 2 | 3 }>();
</script>

<template>
  <div class="flex flex-col items-center gap-3 rounded-box border border-dashed border-base-300 p-8 text-center">
    <Icon :name="icon" size="32" class="text-base-content/70" />
    <component :is="`h${headingLevel}`" class="font-semibold">
      {{ title }}
    </component>
    <p v-if="body" class="max-w-md text-sm text-base-content/70">
      {{ body }}
    </p>
    <div v-if="$slots.actions" class="flex flex-wrap justify-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
