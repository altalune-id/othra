<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue';
import { useToast } from './useToast';

const { lifetime = 5000, dismissLabel = 'Dismiss' } = defineProps<{ lifetime?: number, dismissLabel?: string }>();

const { toasts, dismiss } = useToast();
const timers = new Map<number, ReturnType<typeof setTimeout>>();

function arm(id: number) {
  pause(id);
  timers.set(id, setTimeout(() => {
    timers.delete(id);
    dismiss(id);
  }, lifetime));
}

function pause(id: number) {
  clearTimeout(timers.get(id));
  timers.delete(id);
}

watch(() => toasts.map(t => t.id), (ids) => {
  ids.filter(id => !timers.has(id)).forEach(arm);
}, { immediate: true });

onBeforeUnmount(() => timers.forEach(clearTimeout));
</script>

<template>
  <div class="toast toast-end toast-bottom z-50" aria-live="polite">
    <div
      v-for="t in toasts"
      :key="t.id"
      class="alert"
      :class="`alert-${t.kind}`"
      :role="t.kind === 'error' ? 'alert' : 'status'"
      @mouseenter="pause(t.id)"
      @mouseleave="arm(t.id)"
      @focusin="pause(t.id)"
      @focusout="arm(t.id)"
    >
      <Icon :name="t.kind === 'error' ? 'lucide:circle-x' : t.kind === 'success' ? 'lucide:circle-check' : 'lucide:info'" size="16" />
      <span>{{ t.text }}</span>
      <button type="button" class="btn btn-ghost btn-xs btn-square" :aria-label="dismissLabel" @click="dismiss(t.id)">
        <Icon name="lucide:x" size="14" />
      </button>
    </div>
  </div>
</template>
