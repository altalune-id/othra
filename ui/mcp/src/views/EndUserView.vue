<script setup lang="ts">
import type { Ref } from 'vue';
import type { ShownError } from '#base/lib/apperror/message';
import type { ToolCaller } from '../generic/bridge';
import ErrorText from '#base/components/ui/ErrorText.vue';
import ViewShell from '../components/ViewShell.vue';
import { t } from '../copy';
import { callTool } from '../generic/bridge';
import { objectAt, pick, rowsAt } from '../generic/descriptor';
import FieldList from '../generic/FieldList.vue';
import RowTable from '../generic/RowTable.vue';
import { endUserFields, endUserSections } from './endusers';

const props = defineProps<{ data: Record<string, unknown> }>();
const app = inject<ToolCaller>('mcpApp');
const input = inject<Ref<Record<string, unknown>>>('mcpInput', ref({}));

const state = ref<Record<string, unknown>>(props.data);
const error = ref<ShownError | null>(null);
const user = computed(() => objectAt(state.value, 'endUser'));
const headings = ref<HTMLElement[]>([]);

watch(() => props.data, (fresh) => {
  state.value = fresh;
});

async function refresh(section: number) {
  headings.value[section]?.focus();
  if (!app) {
    return;
  }
  error.value = null;
  try {
    state.value = await callTool(app, 'end_user_get', pick(input.value, ['projectId', 'endUserId']));
  } catch (err) {
    error.value = { message: (err as Error).message };
  }
}
</script>

<template>
  <ViewShell :title="String(user?.email || t('mcp.end_user'))">
    <p v-if="!user" class="text-sm text-base-content/60">
      {{ t('mcp.nothing') }}
    </p>
    <template v-else>
      <FieldList :fields="endUserFields" :object="user" />
      <section v-for="(s, i) in endUserSections" :key="s.rows" class="space-y-1 border-t border-base-300 pt-3">
        <h2 ref="headings" tabindex="-1" class="text-sm font-semibold focus:outline-none">
          {{ s.title }}
        </h2>
        <p v-if="s.help" class="text-xs text-base-content/60">
          {{ s.help }}
        </p>
        <RowTable :rows="rowsAt(state, s.rows)" :columns="s.columns" :row-id="s.key" :actions="s.actions" :empty="s.empty" @done="refresh(i)" />
      </section>
      <ErrorText :error="error" />
    </template>
  </ViewShell>
</template>
