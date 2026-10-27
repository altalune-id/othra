<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import ErrorText from '#base/components/ui/ErrorText.vue';

export interface PersonalKeyCreateLabels {
  name: string;
  org: string;
  orgHelp: string;
  selectOrg: string;
  expires: string;
  expiresHelp: string;
  projects: string;
  allProjects: string;
  selectProjects: string;
  scopes: string;
  scopesHelp: string;
  scopesPickOrg: string;
  cancel: string;
  create: string;
  creating: string;
}

withDefaults(defineProps<{
  orgOptions: { value: string, label: string }[],
  oneOrg?: boolean,
  orgName?: string,
  expiryOptions: { days: number, label: string }[],
  projects: { id: string, name: string }[],
  scopeOptions: { name: string, label: string }[],
  busy?: boolean,
  canSubmit?: boolean,
  error?: ShownError | null,
  cancelUrl?: string,
  labels?: PersonalKeyCreateLabels,
}>(), {
  oneOrg: false,
  orgName: '',
  busy: false,
  canSubmit: false,
  error: null,
  cancelUrl: '/account/api-keys',
  labels: () => ({
    name: 'Name',
    org: 'Organization',
    orgHelp: 'The key works only in this organization, with at most your role there. You can\'t change this later.',
    selectOrg: 'Select organization',
    expires: 'Expires',
    expiresHelp: 'The key stops working on this date. You can\'t extend it; create a new key instead.',
    projects: 'Projects',
    allProjects: 'All projects',
    selectProjects: 'Only select projects',
    scopes: 'Scopes',
    scopesHelp: 'Pick only what the script or client needs. You can grant only what your role allows.',
    scopesPickOrg: 'Select an organization to see the scopes you can grant.',
    cancel: 'Cancel',
    create: 'Create',
    creating: 'Creating…',
  }),
});

const emit = defineEmits<{ selectOrg: [orgId: string], submit: [] }>();

const name = defineModel<string>('name', { default: '' });
const orgId = defineModel<string>('orgId', { default: '' });
const expiresInDays = defineModel<number>('expiresInDays', { default: 30 });
const allProjects = defineModel<boolean>('allProjects', { default: true });
const projectIds = defineModel<string[]>('projectIds', { default: () => [] });
const scopes = defineModel<string[]>('scopes', { default: () => [] });

const id = useId();
const ids = {
  name: `${id}-name`,
  org: `${id}-org`,
  orgHelp: `${id}-org-help`,
  expires: `${id}-expires`,
  expiresHelp: `${id}-expires-help`,
  scopesHelp: `${id}-scopes-help`,
};

function onOrgChange(event: Event) {
  emit('selectOrg', (event.target as HTMLSelectElement).value);
}
</script>

<template>
  <form class="space-y-2" @submit.prevent="emit('submit')">
    <div class="fieldset">
      <label :for="ids.name" class="fieldset-legend">{{ labels.name }}</label>
      <input :id="ids.name" v-model="name" type="text" class="input w-full" placeholder="laptop" maxlength="100" autocomplete="off" :disabled="busy">
    </div>

    <div class="fieldset">
      <template v-if="oneOrg">
        <span class="fieldset-legend">{{ labels.org }}</span>
        <p class="py-2 font-medium">
          {{ orgName }}
        </p>
      </template>
      <template v-else>
        <label :for="ids.org" class="fieldset-legend">{{ labels.org }}</label>
        <select :id="ids.org" class="select w-full" :value="orgId" :aria-describedby="ids.orgHelp" :disabled="busy" @change="onOrgChange">
          <option value="" disabled>
            {{ labels.selectOrg }}
          </option>
          <option v-for="o in orgOptions" :key="o.value" :value="o.value">
            {{ o.label }}
          </option>
        </select>
      </template>
      <p :id="ids.orgHelp" class="label whitespace-normal">
        {{ labels.orgHelp }}
      </p>
    </div>

    <div class="fieldset">
      <label :for="ids.expires" class="fieldset-legend">{{ labels.expires }}</label>
      <select :id="ids.expires" v-model.number="expiresInDays" class="select w-full" :aria-describedby="ids.expiresHelp" :disabled="busy">
        <option v-for="o in expiryOptions" :key="o.days" :value="o.days">
          {{ o.label }}
        </option>
      </select>
      <p :id="ids.expiresHelp" class="label whitespace-normal">
        {{ labels.expiresHelp }}
      </p>
    </div>

    <fieldset class="fieldset">
      <legend class="fieldset-legend">
        {{ labels.projects }}
      </legend>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="allProjects" type="radio" :value="true" class="radio radio-sm" :disabled="busy">
        {{ labels.allProjects }}
      </label>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="allProjects" type="radio" :value="false" class="radio radio-sm" :disabled="busy || !orgId">
        {{ labels.selectProjects }}
      </label>
      <div v-if="!allProjects" class="max-h-40 space-y-1 overflow-auto rounded-box border border-base-300 p-2">
        <label v-for="p in projects" :key="p.id" class="flex items-center gap-2 text-sm">
          <input v-model="projectIds" type="checkbox" :value="p.id" class="checkbox checkbox-sm rounded-sm" :disabled="busy">
          {{ p.name }}
        </label>
      </div>
    </fieldset>

    <fieldset class="fieldset" :aria-describedby="orgId ? ids.scopesHelp : undefined">
      <legend class="fieldset-legend">
        {{ labels.scopes }}
      </legend>
      <p v-if="!orgId" class="text-sm text-base-content/60">
        {{ labels.scopesPickOrg }}
      </p>
      <template v-else>
        <div class="grid gap-1 sm:grid-cols-2">
          <label v-for="s in scopeOptions" :key="s.name" class="flex items-center gap-2 text-sm">
            <input v-model="scopes" type="checkbox" :value="s.name" class="checkbox checkbox-sm rounded-sm" :disabled="busy">
            {{ s.label }}
          </label>
        </div>
        <p :id="ids.scopesHelp" class="label whitespace-normal">
          {{ labels.scopesHelp }}
        </p>
      </template>
    </fieldset>

    <ErrorText :error="error" />
    <div class="flex justify-end gap-2 pt-2">
      <NuxtLink :to="cancelUrl" class="btn btn-ghost btn-sm">
        {{ labels.cancel }}
      </NuxtLink>
      <button type="submit" class="btn btn-primary btn-sm" :disabled="!canSubmit">
        {{ busy ? labels.creating : labels.create }}
      </button>
    </div>
  </form>
</template>
