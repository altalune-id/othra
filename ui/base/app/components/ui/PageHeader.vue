<script setup lang="ts">
import type { Crumb } from './types';

const { title, description = '', mono = false, crumbs = [], crumbsLabel = 'Breadcrumbs', preview = false, previewLabel = 'Preview' } = defineProps<{
  title: string,
  description?: string,
  mono?: boolean,
  crumbs?: Crumb[],
  crumbsLabel?: string,
  preview?: boolean,
  previewLabel?: string,
}>();
</script>

<template>
  <header class="space-y-1">
    <nav v-if="crumbs.length" :aria-label="crumbsLabel" class="breadcrumbs py-0 text-sm text-base-content/70">
      <ul>
        <li v-for="(crumb, i) in crumbs" :key="i">
          <NuxtLink v-if="crumb.to && i < crumbs.length - 1" :to="crumb.to" class="link link-hover">
            {{ crumb.label }}
          </NuxtLink>
          <span v-else :aria-current="i === crumbs.length - 1 ? 'page' : undefined">{{ crumb.label }}</span>
        </li>
      </ul>
    </nav>
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0 space-y-1">
        <div class="flex flex-wrap items-center gap-2">
          <h1 :class="mono ? 'font-mono text-xl font-semibold break-all' : 'text-2xl font-semibold'">
            {{ title }}
          </h1>
          <span v-if="preview" class="badge badge-soft badge-info badge-sm">{{ previewLabel }}</span>
        </div>
        <p v-if="description" class="text-sm text-base-content/70">
          {{ description }}
        </p>
        <slot name="meta" />
      </div>
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </header>
</template>
