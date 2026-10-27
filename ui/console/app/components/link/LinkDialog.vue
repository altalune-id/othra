<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';

export interface LinkDialogLabels {
  title: string;
  target: string;
  scopes: string;
  scopesHelp: string;
  pickScope: string;
  unverified: string;
  noScopes: string;
  noneLeft: string;
  cancel: string;
  confirm: string;
}

const { open, mode = 'new', targets, fixedName = '', scopeOptions, pickError = false, unverified = false, busy = false, error = null, labels = {
  title: 'Link a resource server',
  target: 'Resource server',
  scopes: 'Allowed scopes',
  scopesHelp: 'The client can ask only for these. Pick at least one.',
  pickScope: 'Pick at least one scope.',
  unverified: 'This server isn\'t verified, so the client can\'t get tokens for it yet.',
  noScopes: 'Add scopes to this resource server first.',
  noneLeft: 'Everything here is already linked.',
  cancel: 'Cancel',
  confirm: 'Link',
} } = defineProps<{
  open: boolean,
  mode?: 'new' | 'edit',
  targets: { value: string, label: string }[],
  fixedName?: string,
  scopeOptions: string[],
  pickError?: boolean,
  unverified?: boolean,
  busy?: boolean,
  error?: ShownError | null,
  labels?: LinkDialogLabels,
}>();

const emit = defineEmits<{ confirm: [], close: [], target: [id: string] }>();
const targetId = defineModel<string>('targetId', { required: true });
const picked = defineModel<string[]>('picked', { required: true });
const ids = { target: useId(), scopes: useId(), err: useId() };
</script>

<template>
  <AppModal :open="open" :title="labels.title" @close="emit('close')">
    <div class="space-y-3">
      <p v-if="mode === 'new' && targets.length === 0" class="text-sm text-base-content/70">
        {{ labels.noneLeft }}
      </p>
      <template v-else>
        <div class="fieldset">
          <label :for="ids.target" class="fieldset-legend">{{ labels.target }}</label>
          <p v-if="mode === 'edit'" :id="ids.target" class="text-sm font-medium">
            {{ fixedName }}
          </p>
          <select
            v-else
            :id="ids.target"
            :value="targetId"
            class="select w-full border-base-300"
            :disabled="busy"
            @change="emit('target', ($event.target as HTMLSelectElement).value)"
          >
            <option v-for="o in targets" :key="o.value" :value="o.value">
              {{ o.label }}
            </option>
          </select>
        </div>
        <p v-if="unverified" class="flex items-start gap-1.5 text-sm text-base-content">
          <Icon name="lucide:info" size="16" class="mt-0.5 shrink-0 text-info" />
          {{ labels.unverified }}
        </p>
        <fieldset class="fieldset" :aria-describedby="`${ids.scopes} ${ids.err}`">
          <legend class="fieldset-legend">
            {{ labels.scopes }}
          </legend>
          <p v-if="scopeOptions.length === 0" class="text-sm text-base-content/70">
            {{ labels.noScopes }}
          </p>
          <div v-else class="space-y-1">
            <label v-for="scope in scopeOptions" :key="scope" class="flex items-center gap-2 text-sm">
              <input v-model="picked" type="checkbox" :value="scope" class="checkbox checkbox-sm rounded-sm" :disabled="busy">
              <span class="font-mono">{{ scope }}</span>
            </label>
          </div>
          <FieldError :id="ids.err" :message="pickError ? labels.pickScope : ''" />
          <p :id="ids.scopes" class="label whitespace-normal">
            {{ labels.scopesHelp }}
          </p>
        </fieldset>
      </template>
      <ErrorText :error="error" />
    </div>
    <template #footer>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="emit('close')">
        {{ labels.cancel }}
      </button>
      <button type="button" class="btn btn-primary btn-sm" :disabled="busy || scopeOptions.length === 0 || (mode === 'new' && targets.length === 0)" @click="emit('confirm')">
        <span v-if="busy" class="loading loading-spinner loading-xs" />
        {{ labels.confirm }}
      </button>
    </template>
  </AppModal>
</template>
