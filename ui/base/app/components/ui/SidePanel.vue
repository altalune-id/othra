<script setup lang="ts">
import { inject, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue';
import { shellHoldKey } from './shellHold';

const { title, closeLabel = 'Close' } = defineProps<{ title: string, closeLabel?: string }>();

const open = defineModel<boolean>('open', { default: false });

const toggleId = `panel-${useId()}`;
const titleId = `${toggleId}-title`;
const close = ref<HTMLButtonElement>();
let returnTo: HTMLElement | null = null;
const shell = inject(shellHoldKey, null);
let holding = false;

function setHold(on: boolean) {
  if (on === holding) {
    return;
  }
  holding = on;
  if (on) {
    shell?.hold();
  } else {
    shell?.release();
  }
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    open.value = false;
  }
}

async function sync(now: boolean) {
  setHold(now);
  if (now) {
    returnTo = document.activeElement as HTMLElement | null;
    document.addEventListener('keydown', onKey);
    close.value?.focus();
    return;
  }
  document.removeEventListener('keydown', onKey);
  await nextTick();
  returnTo?.focus();
  returnTo = null;
}

watch(open, sync, { flush: 'post' });
onMounted(() => {
  if (open.value) {
    void sync(true);
  }
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey);
  setHold(false);
});
</script>

<template>
  <Teleport to="body">
    <div class="drawer drawer-end">
      <input :id="toggleId" v-model="open" type="checkbox" class="drawer-toggle" tabindex="-1" aria-hidden="true">
      <div class="drawer-side z-50">
        <label :for="toggleId" :aria-label="closeLabel" class="drawer-overlay" />
        <aside role="dialog" aria-modal="true" :aria-labelledby="titleId" class="flex min-h-full w-96 max-w-[calc(100vw-2.5rem)] flex-col gap-3 border-s border-base-300 bg-base-100 p-4">
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
  </Teleport>
</template>
