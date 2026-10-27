<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import UrlField from '#base/components/ui/UrlField.vue';

export interface GoogleUpstreamView {
  clientId: string;
  secretSet: boolean;
  enabled: boolean;
}

export interface GoogleUpstreamCardLabels {
  title: string;
  upstream: string;
  intro: string;
  setup: string;
  redirectUri: string;
  redirectHelp: string;
  copy: string;
  clientId: string;
  clientIdHelp: string;
  clientSecret: string;
  secretHelp: string;
  secretSet: string;
  secretReplace: string;
  enabled: string;
  enabledHelp: string;
  locked: string;
  save: string;
  saving: string;
  saved: string;
  cancel: string;
  remove: string;
  removeLocked: string;
  turnOffInstead: string;
  on: string;
  off: string;
}

const { canEdit = false, google = null, settingUp = false, redirectUri = '', busy = false, saved = false, error = null, toggleError = null, fieldErrors = {}, locked = false, removeLocked = false, labels = {
  title: 'Google',
  upstream: 'Upstream IdP',
  intro: 'Let end users sign in with their Google account.',
  setup: 'Set up Google',
  redirectUri: 'Redirect URI to register',
  redirectHelp: 'Add this exact URI under Authorized redirect URIs in your Google Cloud OAuth client.',
  copy: 'Copy {label}',
  clientId: 'Client ID',
  clientIdHelp: 'From Google Cloud Console → APIs & Services → Credentials.',
  clientSecret: 'Client secret',
  secretHelp: 'Stored encrypted. It isn\'t shown again.',
  secretSet: 'Set',
  secretReplace: 'Replace',
  enabled: 'Enabled',
  enabledHelp: 'Off hides the Google button on the sign-in page.',
  locked: 'Turn the email link on first: Google is the only way to sign in.',
  save: 'Save',
  saving: 'Saving…',
  saved: 'Saved',
  cancel: 'Cancel',
  remove: 'Remove Google',
  removeLocked: 'End users have linked Google, so it can\'t be removed.',
  turnOffInstead: 'Turn it off instead.',
  on: 'On',
  off: 'Off',
} } = defineProps<{
  canEdit?: boolean,
  google?: GoogleUpstreamView | null,
  settingUp?: boolean,
  redirectUri?: string,
  busy?: boolean,
  saved?: boolean,
  error?: ShownError | null,
  toggleError?: ShownError | null,
  fieldErrors?: Record<string, string>,
  locked?: boolean,
  removeLocked?: boolean,
  labels?: GoogleUpstreamCardLabels,
}>();

const emit = defineEmits<{ setup: [], cancel: [], save: [], toggle: [on: boolean], remove: [] }>();
const clientId = defineModel<string>('clientId', { required: true });
const secret = defineModel<string>('secret', { required: true });
const replacingSecret = defineModel<boolean>('replacingSecret', { default: false });
const ids = { enabled: useId(), enabledHelp: useId(), locked: useId(), clientId: useId(), clientIdHelp: useId(), clientIdErr: useId(), secret: useId(), secretHelp: useId(), secretErr: useId(), removeLocked: useId() };

function onToggle(event: Event) {
  const input = event.target as HTMLInputElement;
  emit('toggle', input.checked);
  input.checked = google?.enabled ?? false;
}
</script>

<template>
  <section class="card border border-base-300 bg-base-100" data-testid="google-upstream-card">
    <div class="card-body gap-3">
      <div>
        <h2 class="card-title text-base">
          {{ labels.title }}
        </h2>
        <p class="text-xs text-base-content/70">
          {{ labels.upstream }}
        </p>
      </div>

      <template v-if="!google && !settingUp">
        <p class="text-sm text-base-content/70">
          {{ labels.intro }}
        </p>
        <div v-if="canEdit">
          <button type="button" class="btn btn-primary btn-sm" @click="emit('setup')">
            {{ labels.setup }}
          </button>
        </div>
      </template>

      <template v-else>
        <UrlField :label="labels.redirectUri" :value="redirectUri" :help="labels.redirectHelp" :copy-label="labels.copy" testid="google-redirect-uri" />

        <form v-if="canEdit" novalidate class="space-y-3" @submit.prevent="emit('save')">
          <div class="fieldset">
            <label :for="ids.clientId" class="fieldset-legend">{{ labels.clientId }}</label>
            <input
              :id="ids.clientId"
              v-model="clientId"
              class="input w-full font-mono text-sm"
              :class="{ 'input-error': fieldErrors.client_id }"
              autocomplete="off"
              spellcheck="false"
              :disabled="busy"
              :aria-invalid="!!fieldErrors.client_id"
              :aria-describedby="`${ids.clientIdHelp} ${ids.clientIdErr}`"
            >
            <FieldError :id="ids.clientIdErr" :message="fieldErrors.client_id" />
            <p :id="ids.clientIdHelp" class="label whitespace-normal">
              {{ labels.clientIdHelp }}
            </p>
          </div>
          <div class="fieldset">
            <label :for="ids.secret" class="fieldset-legend">{{ labels.clientSecret }}</label>
            <div v-if="google?.secretSet && !replacingSecret" class="flex items-center gap-3 text-sm">
              <span class="flex items-center gap-1" data-testid="secret-set">
                <Icon name="lucide:lock" size="14" class="shrink-0" />
                {{ labels.secretSet }}
              </span>
              <button type="button" class="btn btn-ghost btn-xs" :disabled="busy" @click="replacingSecret = true">
                {{ labels.secretReplace }}
              </button>
            </div>
            <template v-else>
              <input
                :id="ids.secret"
                v-model="secret"
                type="password"
                class="input w-full font-mono text-sm"
                :class="{ 'input-error': fieldErrors.client_secret }"
                autocomplete="off"
                spellcheck="false"
                :disabled="busy"
                :aria-invalid="!!fieldErrors.client_secret"
                :aria-describedby="`${ids.secretHelp} ${ids.secretErr}`"
              >
              <FieldError :id="ids.secretErr" :message="fieldErrors.client_secret" />
            </template>
            <p :id="ids.secretHelp" class="label whitespace-normal">
              {{ labels.secretHelp }}
            </p>
          </div>
          <ErrorText :error="error" />
          <div class="flex flex-wrap items-center gap-3 pt-1">
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
              {{ busy ? labels.saving : labels.save }}
            </button>
            <button v-if="!google" type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="emit('cancel')">
              {{ labels.cancel }}
            </button>
            <SavedNote v-if="saved" :label="labels.saved" />
          </div>
        </form>
        <dl v-else-if="google" class="space-y-1 text-sm" data-testid="google-readonly">
          <dt class="fieldset-legend">
            {{ labels.clientId }}
          </dt>
          <dd class="font-mono break-all">
            {{ google.clientId }}
          </dd>
          <dt class="fieldset-legend">
            {{ labels.enabled }}
          </dt>
          <dd>{{ google.enabled ? labels.on : labels.off }}</dd>
        </dl>

        <div v-if="canEdit && google" class="space-y-3 border-t border-base-300 pt-3">
          <div class="space-y-1">
            <label :for="ids.enabled" class="flex items-center gap-3 text-sm font-medium">
              <input
                :id="ids.enabled"
                :checked="google.enabled"
                type="checkbox"
                class="toggle toggle-sm"
                :class="{ 'cursor-not-allowed': locked }"
                :disabled="busy"
                :aria-disabled="locked"
                :aria-describedby="locked ? `${ids.enabledHelp} ${ids.locked}` : ids.enabledHelp"
                @click="locked && $event.preventDefault()"
                @change="onToggle"
              >
              {{ labels.enabled }}
            </label>
            <p :id="ids.enabledHelp" class="label whitespace-normal">
              {{ labels.enabledHelp }}
            </p>
            <p v-if="locked" :id="ids.locked" class="flex items-start gap-1 text-xs text-base-content">
              <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
              {{ labels.locked }}
            </p>
            <ErrorText :error="toggleError" />
          </div>
          <div class="space-y-1">
            <button type="button" class="btn btn-ghost btn-sm" :disabled="busy || locked || removeLocked" :aria-describedby="removeLocked ? ids.removeLocked : undefined" @click="emit('remove')">
              <Icon name="lucide:trash-2" size="16" class="text-error" />
              {{ labels.remove }}
            </button>
            <p v-if="removeLocked" :id="ids.removeLocked" class="flex items-start gap-1 text-xs text-base-content">
              <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
              {{ google.enabled && !locked ? `${labels.removeLocked} ${labels.turnOffInstead}` : labels.removeLocked }}
            </p>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>
