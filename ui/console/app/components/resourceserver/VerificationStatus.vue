<script setup lang="ts">
import type { VerificationName } from './verification';
import { verificationDot } from './verification';

defineProps<{ name: VerificationName, label: string }>();
</script>

<template>
  <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-sm">
    <span class="status" :class="verificationDot(name)" aria-hidden="true" />
    {{ label }}
  </span>
</template>
