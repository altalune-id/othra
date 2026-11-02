<script setup lang="ts">
import { onBeforeUnmount, ref, useId, watch } from 'vue';

const { title, closeLabel = 'Close' } = defineProps<{ title: string, closeLabel?: string }>();

const open = defineModel<boolean>('open', { default: false });

const toggleId = `panel-${useId()}`;
const titleId = `${toggleId}-title`;
const close = ref<HTMLButtonElement>();
let returnTo: HTMLElement | null = null;

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    open.value = false;
  }
}

watch(open, (now) => {
  if (now) {
    returnTo = document.activeElement as HTMLElement | null;
    document.addEventListener('keydown', onKey);
    close.value?.focus();
    return;
  }
  document.removeEventListener('keydown', onKey);
  returnTo?.focus();
  returnTo = null;
}, { flush: 'post' });

onBeforeUnmount(() => document.removeEventListener('keydown', onKey));
</script>

<template>
  <div class="drawer drawer-end">
    <input :id="toggleId" v-model="open" type="checkbox" class="drawer-toggle" tabindex="-1" aria-hidden="true" :aria-label="title">
    <div class="drawer-side z-50">
      <label :for="toggleId" :aria-label="closeLabel" class="drawer-overlay" />
      <aside role="dialog" :aria-labelledby="titleId" class="flex min-h-full w-96 max-w-[calc(100vw-2.5rem)] flex-col gap-3 border-s border-base-300 bg-base-100 p-4">
        <div class="flex items-center justify-between gap-2">
          <h2 :id="titleId" class="text-sm font-semibold">
            {{ title }}
          </h2>
          <button ref="close" type="button" class="btn btn-sm btn-ghost btn-square text-ui" :aria-label="closeLabel" @click="open = false">
            <Icon name="lucide:x" size="16" class="shrink-0" aria-hidden="true" />
          </button>
        </div>
        <slot />
      </aside>
    </div>
  </div>
</template>
