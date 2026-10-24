<script setup lang="ts">
import type { ShownError } from '../../lib/apperror/message';
import { useId } from 'vue';
import ErrorText from '../ui/ErrorText.vue';
import FieldError from '../ui/FieldError.vue';
import SavedNote from '../ui/SavedNote.vue';

export interface CimdApp {
  id: string;
  name: string;
  url: string;
  firstSeen: string;
  fetched: string;
}

export interface CimdCardLabels {
  title: string;
  accept: string;
  acceptHelp: string;
  locked: string;
  hosts: string;
  hostsHelp: string;
  hostsAny: string;
  scopes: string;
  scopesAll: string;
  scopesSome: string;
  save: string;
  saved: string;
  apps: string;
  appsEmpty: string;
  firstSeen: string;
  fetched: string;
  revoke: string;
  on: string;
  off: string;
}

const { canEdit = false, locked = false, catalog = [], apps = [], busy = false, saved = false, error = null, hostsError = '', labels = {
  title: 'Apps without registration (CIMD)',
  accept: 'Accept apps that aren\'t registered',
  acceptHelp: 'MCP clients such as Claude Code sign in with a metadata document they host, without being registered here. End users always see a consent page that names the app\'s host. Turning this on also tells MCP clients that this project accepts them.',
  locked: 'Verify this server first. Apps can\'t get tokens for a server that isn\'t verified.',
  hosts: 'Allowed hosts',
  hostsHelp: 'One host per line, for example claude.ai. Only apps whose client ID is on these hosts can sign in.',
  hostsAny: 'Empty means any host: anyone who can publish an https document can ask for this server\'s scopes.',
  scopes: 'Scopes for these apps',
  scopesAll: 'All scopes of this server',
  scopesSome: 'Only these',
  save: 'Save',
  saved: 'Saved',
  apps: 'Apps that signed in',
  appsEmpty: 'No app has signed in this way yet.',
  firstSeen: 'First seen',
  fetched: 'Details updated',
  revoke: 'Revoke',
  on: 'On',
  off: 'Off',
} } = defineProps<{
  canEdit?: boolean,
  locked?: boolean,
  catalog?: string[],
  apps?: CimdApp[],
  busy?: boolean,
  saved?: boolean,
  error?: ShownError | null,
  hostsError?: string,
  labels?: CimdCardLabels,
}>();

const emit = defineEmits<{ save: [], revoke: [app: CimdApp] }>();
const accept = defineModel<boolean>('accept', { required: true });
const hosts = defineModel<string>('hosts', { required: true });
const allScopes = defineModel<boolean>('allScopes', { required: true });
const picked = defineModel<string[]>('picked', { required: true });
const ids = { accept: useId(), help: useId(), hosts: useId(), hostsHelp: useId(), hostsErr: useId() };
</script>

<template>
  <section class="card border border-base-300 bg-base-100" data-testid="cimd-card">
    <div class="card-body gap-3">
      <h2 class="card-title text-base">
        {{ labels.title }}
      </h2>
      <form v-if="canEdit" novalidate class="space-y-3" @submit.prevent="emit('save')">
        <div class="space-y-1">
          <label :for="ids.accept" class="flex items-center gap-3 text-sm font-medium">
            <input
              :id="ids.accept"
              v-model="accept"
              type="checkbox"
              class="toggle toggle-sm"
              :disabled="busy || (locked && !accept)"
              :aria-describedby="ids.help"
            >
            {{ labels.accept }}
          </label>
          <p :id="ids.help" class="label whitespace-normal">
            {{ labels.acceptHelp }}
          </p>
          <p v-if="locked && !accept" class="flex items-start gap-1 text-xs text-base-content">
            <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
            {{ labels.locked }}
          </p>
        </div>
        <template v-if="accept">
          <div class="fieldset">
            <label :for="ids.hosts" class="fieldset-legend">{{ labels.hosts }}</label>
            <textarea
              :id="ids.hosts"
              v-model="hosts"
              class="textarea w-full font-mono text-sm"
              :class="{ 'textarea-error': hostsError }"
              rows="3"
              placeholder="claude.ai"
              autocomplete="off"
              spellcheck="false"
              :disabled="busy"
              :aria-invalid="!!hostsError"
              :aria-describedby="`${ids.hostsHelp} ${ids.hostsErr}`"
            />
            <FieldError :id="ids.hostsErr" :message="hostsError" />
            <p v-if="hosts.trim() === ''" class="mt-1 flex items-start gap-1 text-xs text-base-content">
              <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" />
              {{ labels.hostsAny }}
            </p>
            <p :id="ids.hostsHelp" class="label whitespace-normal">
              {{ labels.hostsHelp }}
            </p>
          </div>
          <fieldset class="fieldset">
            <legend class="fieldset-legend">
              {{ labels.scopes }}
            </legend>
            <label class="flex items-center gap-2 text-sm">
              <input type="radio" name="cimd-scopes" class="radio radio-sm" :checked="allScopes" :disabled="busy" @change="allScopes = true">
              {{ labels.scopesAll }}
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input type="radio" name="cimd-scopes" class="radio radio-sm" :checked="!allScopes" :disabled="busy" @change="allScopes = false">
              {{ labels.scopesSome }}
            </label>
            <div v-if="!allScopes" class="mt-1 space-y-1 pl-6">
              <label v-for="scope in catalog" :key="scope" class="flex items-center gap-2 text-sm">
                <input v-model="picked" type="checkbox" :value="scope" class="checkbox checkbox-sm rounded-sm" :disabled="busy">
                <span class="font-mono">{{ scope }}</span>
              </label>
            </div>
          </fieldset>
        </template>
        <ErrorText :error="error" />
        <div class="flex items-center gap-3">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-xs" />
            {{ labels.save }}
          </button>
          <SavedNote v-if="saved" :label="labels.saved" />
        </div>
      </form>
      <dl v-else class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt class="text-base-content/60">
          {{ labels.accept }}
        </dt>
        <dd>{{ accept ? labels.on : labels.off }}</dd>
        <template v-if="accept">
          <dt class="text-base-content/60">
            {{ labels.hosts }}
          </dt>
          <dd class="font-mono text-xs break-all">
            {{ hosts.trim() ? hosts.split('\n').join(', ') : '—' }}
          </dd>
          <dt class="text-base-content/60">
            {{ labels.scopes }}
          </dt>
          <dd>{{ allScopes ? labels.scopesAll : picked.join(', ') }}</dd>
        </template>
      </dl>

      <div class="divider my-0" />
      <h3 class="text-sm font-medium">
        {{ labels.apps }}
      </h3>
      <p v-if="apps.length === 0" class="text-sm text-base-content/70">
        {{ labels.appsEmpty }}
      </p>
      <ul v-else class="divide-y divide-base-300">
        <li v-for="app in apps" :key="app.id" class="flex flex-wrap items-start justify-between gap-2 py-2">
          <div class="min-w-0 space-y-0.5">
            <p class="text-sm font-medium break-words">
              {{ app.name }}
            </p>
            <p class="font-mono text-xs break-all text-base-content/70">
              {{ app.url }}
            </p>
            <p class="text-xs text-base-content/70">
              {{ labels.firstSeen }} {{ app.firstSeen }}<template v-if="app.fetched">
                · {{ labels.fetched }} {{ app.fetched }}
              </template>
            </p>
          </div>
          <button v-if="canEdit" type="button" class="btn btn-ghost btn-xs" @click="emit('revoke', app)">
            <Icon name="lucide:ban" size="14" class="text-error" />
            {{ labels.revoke }}
          </button>
        </li>
      </ul>
    </div>
  </section>
</template>
