<script setup lang="ts">
import type { ShownError } from '../../../lib/apperror/message';
import type { ExpiryState } from './keyStatus';
import { ref } from 'vue';
import CopyButton from '../../ui/CopyButton.vue';
import ErrorText from '../../ui/ErrorText.vue';
import SecretField from '../../ui/SecretField.vue';
import { keyStatus } from './keyStatus';

export interface PersonalKeyView {
  rowKey: string;
  orgId: string;
  orgName: string;
  name: string;
  secretHint: string;
  scopes: string[];
  lastUsed: string;
  expires: string;
  expiry: ExpiryState;
  revoked: boolean;
}

export interface PersonalKeysLabels {
  name: string;
  org: string;
  scopes: string;
  lastUsed: string;
  neverUsed: string;
  expires: string;
  expiresSoon: (days: number) => string;
  expired: string;
  status: string;
  active: string;
  revoked: string;
  revoke: string;
  revokeConfirm: string;
  cancel: string;
  empty: string;
  newKey: string;
  loadFailed: (org: string) => string;
  retry: string;
  secretWarning: string;
  copy: string;
  snippet: string;
  copySnippet: string;
  done: string;
  signIn: string;
  signInLink: string;
}

withDefaults(defineProps<{
  keys: PersonalKeyView[],
  oneOrg?: boolean,
  failed?: { id: string, name: string }[],
  created?: { plaintext: string, snippet: string } | null,
  error?: ShownError | null,
  signedIn?: boolean,
  loginUrl?: string,
  newUrl?: string,
  labels?: PersonalKeysLabels,
}>(), {
  oneOrg: false,
  failed: () => [],
  created: null,
  error: null,
  signedIn: true,
  loginUrl: '/login',
  newUrl: '/account/api-keys/new',
  labels: () => ({
    name: 'Name',
    org: 'Organization',
    scopes: 'Scopes',
    lastUsed: 'Last used',
    neverUsed: 'Never used',
    expires: 'Expires',
    expiresSoon: (days: number) => `Expires in ${days} days`,
    expired: 'Expired',
    status: 'Status',
    active: 'Active',
    revoked: 'Revoked',
    revoke: 'Revoke',
    revokeConfirm: 'Revoke this key?',
    cancel: 'Cancel',
    empty: 'You have no personal API keys yet. Create one to use the CLI or an MCP client as yourself.',
    newKey: 'New personal API key',
    loadFailed: (org: string) => `Couldn't load keys for ${org}.`,
    retry: 'Retry',
    secretWarning: 'Copy this secret now — it will not be shown again.',
    copy: 'Copy',
    snippet: 'Add this to .mcp.json',
    copySnippet: 'Copy snippet',
    done: 'Done',
    signIn: 'Sign in to manage your personal API keys.',
    signInLink: 'Sign in',
  }),
});

const emit = defineEmits<{
  revoke: [orgId: string, hint: string],
  retry: [orgId: string],
  dismiss: [],
}>();

const confirming = ref('');
const statusBadge = { active: 'badge-success', expired: 'badge-error', revoked: 'badge-ghost' } as const;

function revoke(k: PersonalKeyView) {
  confirming.value = '';
  emit('revoke', k.orgId, k.secretHint);
}
</script>

<template>
  <div v-if="!signedIn" class="rounded-box border border-base-300 p-4 text-sm">
    {{ labels.signIn }}
    <a :href="loginUrl" class="link link-primary">{{ labels.signInLink }}</a>
  </div>

  <div v-else class="space-y-4">
    <div v-if="created" role="alert" class="alert alert-success alert-soft flex flex-col items-stretch gap-3">
      <div class="flex w-full items-center justify-between gap-3">
        <p class="text-sm font-medium">
          {{ labels.secretWarning }}
        </p>
        <button type="button" class="btn btn-sm shrink-0" @click="emit('dismiss')">
          {{ labels.done }}
        </button>
      </div>
      <div class="w-full min-w-0">
        <SecretField :value="created.plaintext" :label="labels.copy" />
      </div>
      <div class="w-full min-w-0 space-y-1">
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs">{{ labels.snippet }}</span>
          <CopyButton :value="created.snippet" :label="labels.copySnippet" />
        </div>
        <pre class="overflow-x-auto rounded-box bg-base-100 p-3 text-xs text-base-content">{{ created.snippet }}</pre>
      </div>
    </div>

    <div v-for="org in failed" :key="org.id" role="alert" class="alert alert-warning alert-soft">
      <Icon name="lucide:triangle-alert" size="18" />
      <span class="text-sm">{{ labels.loadFailed(org.name) }}</span>
      <button type="button" class="btn btn-sm" @click="emit('retry', org.id)">
        {{ labels.retry }}
      </button>
    </div>

    <ErrorText :error="error" />

    <div v-if="keys.length === 0" class="flex flex-col items-center gap-3 rounded-box border border-base-300 bg-base-100 px-4 py-10 text-center text-sm text-base-content/60">
      <Icon name="lucide:key-round" size="28" />
      <p>{{ labels.empty }}</p>
      <NuxtLink :to="newUrl" class="btn btn-primary btn-sm">
        <Icon name="lucide:plus" size="16" />
        {{ labels.newKey }}
      </NuxtLink>
    </div>
    <div v-else class="overflow-x-auto rounded-box border border-base-content/5 bg-base-100">
      <table class="table table-sm">
        <thead>
          <tr>
            <th>{{ labels.name }}</th>
            <th v-if="!oneOrg">
              {{ labels.org }}
            </th>
            <th>{{ labels.scopes }}</th>
            <th>{{ labels.lastUsed }}</th>
            <th>{{ labels.expires }}</th>
            <th>{{ labels.status }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in keys" :key="k.rowKey" class="hover:bg-base-200">
            <td class="whitespace-nowrap">
              <span class="font-medium">{{ k.name }}</span>
              <span class="ml-2 font-mono text-xs text-base-content/60">…{{ k.secretHint }}</span>
            </td>
            <td v-if="!oneOrg">
              {{ k.orgName }}
            </td>
            <td class="text-base-content/60">
              {{ k.scopes.join(', ') }}
            </td>
            <td class="whitespace-nowrap text-base-content/60">
              {{ k.lastUsed || labels.neverUsed }}
            </td>
            <td class="whitespace-nowrap">
              <span v-if="k.revoked || k.expiry.kind === 'none'" class="text-base-content/60">—</span>
              <span v-else-if="k.expiry.kind === 'soon'" class="badge badge-sm badge-warning">{{ labels.expiresSoon(k.expiry.days) }}</span>
              <span v-else class="text-base-content/60">{{ k.expires }}</span>
            </td>
            <td>
              <span class="badge badge-sm" :class="statusBadge[keyStatus(k.revoked, k.expiry)]">
                {{ labels[keyStatus(k.revoked, k.expiry)] }}
              </span>
            </td>
            <td class="whitespace-nowrap text-right">
              <template v-if="keyStatus(k.revoked, k.expiry) === 'active'">
                <template v-if="confirming === k.rowKey">
                  <span class="mr-1 text-xs">{{ labels.revokeConfirm }}</span>
                  <button type="button" class="btn btn-error btn-xs" @click="revoke(k)">
                    {{ labels.revoke }}
                  </button>
                  <button type="button" class="btn btn-ghost btn-xs" @click="confirming = ''">
                    {{ labels.cancel }}
                  </button>
                </template>
                <button v-else type="button" class="btn btn-ghost btn-xs" @click="confirming = k.rowKey">
                  <Icon name="lucide:ban" size="14" class="text-error" />
                  {{ labels.revoke }}
                </button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
