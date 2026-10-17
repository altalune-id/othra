<script setup lang="ts">
import { computed } from 'vue';
import { splitIssuerUrl } from './issuerUrl';

const { url } = defineProps<{ url: string }>();

const parts = computed(() => splitIssuerUrl(url));
</script>

<template>
  <code class="min-w-0 flex-1 font-mono text-sm break-words">{{ parts.base }}<wbr>{{ parts.handle }}</code>
</template>
