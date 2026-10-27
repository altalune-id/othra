<script setup lang="ts">
import type { ToolCaller } from './bridge';
import type { BadgeTone, Field } from './descriptor';
import CopyButton from '#base/components/ui/CopyButton.vue';
import SecretField from '#base/components/ui/SecretField.vue';
import { t } from '../copy';
import { openLink } from './bridge';
import { badgeFor, formatValue, isEmpty } from './descriptor';

const { field, value } = defineProps<{ field: Field, value: unknown }>();
const app = inject<ToolCaller | undefined>('mcpApp', undefined);

const tones: Record<BadgeTone, string> = {
  success: 'badge-soft badge-success',
  warning: 'badge-soft badge-warning',
  error: 'badge-soft badge-error',
  info: 'badge-soft badge-info',
  neutral: 'badge-outline',
  ghost: 'badge-ghost',
};

const text = computed(() => formatValue(field, value));
const badge = computed(() => badgeFor(field, value));
const present = computed(() => !isEmpty(value));
const copyLabel = computed(() => t('common.copy_label', { label: field.label }));
const refused = ref(false);

async function open() {
  refused.value = !(await openLink(app, String(value)));
}
</script>

<template>
  <template v-if="field.kind === 'badge'">
    <span v-if="badge.label" class="badge badge-sm" :class="tones[badge.tone]">{{ badge.label }}</span>
  </template>
  <ul v-else-if="field.kind === 'list' && Array.isArray(value) && value.length" class="space-y-0.5">
    <li v-for="item in value" :key="String(item)">
      <code class="font-mono text-xs break-all">{{ item }}</code>
    </li>
  </ul>
  <span v-else-if="(field.kind === 'id' || field.kind === 'url') && present" class="inline-flex max-w-full items-center gap-1">
    <code class="min-w-0 font-mono text-xs break-all">{{ text }}</code>
    <CopyButton :value="String(value)" :label="copyLabel" class="shrink-0" />
  </span>
  <span v-else-if="field.kind === 'link' && present" class="inline-flex flex-col items-start gap-1">
    <button type="button" class="btn btn-link btn-xs h-auto min-h-0 gap-1 px-0" @click="open">
      {{ field.label }}
      <Icon name="lucide:external-link" size="12" />
    </button>
    <span aria-live="polite">
      <span v-if="refused" class="inline-flex max-w-full items-center gap-1 text-xs text-base-content/70">
        <code class="min-w-0 font-mono break-all">{{ text }}</code>
        <CopyButton :value="String(value)" :label="copyLabel" class="shrink-0" />
      </span>
    </span>
  </span>
  <SecretField v-else-if="field.kind === 'secret' && present" :value="String(value)" :label="copyLabel" />
  <code v-else-if="field.kind === 'mono'" class="font-mono text-xs break-all">{{ text }}</code>
  <span v-else>{{ text }}</span>
</template>
