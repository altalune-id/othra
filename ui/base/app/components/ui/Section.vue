<script setup lang="ts">
import { useId } from 'vue';

const { title, description = '', tone } = defineProps<{ title: string, description?: string, tone?: 'danger' }>();

const titleId = useId();
</script>

<template>
  <section class="card card-border bg-base-100" :class="tone === 'danger' ? 'border-error' : 'border-base-300'" :aria-labelledby="titleId">
    <div class="card-body gap-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 :id="titleId" class="card-title text-base">
            {{ title }}
          </h2>
          <p v-if="description" class="text-sm text-base-content/70">
            {{ description }}
          </p>
        </div>
        <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
          <slot name="actions" />
        </div>
      </div>
      <slot />
    </div>
  </section>
</template>
