<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import { useId } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import Section from '#base/components/ui/Section.vue';
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
  <Section :title="labels.title" :description="labels.upstream" data-testid="google-upstream-card">
    <template v-if="!google && !settingUp">
      <p class="text-ui text-base-content/70">
        {{ labels.intro }}
      </p>
      <div v-if="canEdit">
        <AppButton size="sm" @click="emit('setup')">
          {{ labels.setup }}
        </AppButton>
      </div>
    </template>

    <template v-else>
      <UrlField :label="labels.redirectUri" :value="redirectUri" :help="labels.redirectHelp" :copy-label="labels.copy" testid="google-redirect-uri" />

      <form v-if="canEdit" novalidate class="space-y-3" @submit.prevent="emit('save')">
        <div class="fieldset">
          <label class="flex flex-col gap-1">
            <span class="label text-ui text-base-content">{{ labels.clientId }}</span>
            <input
              :id="ids.clientId"
              v-model="clientId"
              class="input input-sm w-full font-mono text-ui"
              :class="{ 'input-error': fieldErrors.client_id }"
              autocomplete="off"
              spellcheck="false"
              :disabled="busy"
              :aria-invalid="!!fieldErrors.client_id"
              :aria-describedby="`${ids.clientIdHelp} ${ids.clientIdErr}`"
            >
          </label>
          <FieldError :id="ids.clientIdErr" :message="fieldErrors.client_id" />
          <p :id="ids.clientIdHelp" class="label whitespace-normal text-xs text-base-content/70">
            {{ labels.clientIdHelp }}
          </p>
        </div>
        <div class="fieldset">
          <div v-if="google?.secretSet && !replacingSecret" class="flex flex-col gap-1">
            <span class="label text-ui text-base-content">{{ labels.clientSecret }}</span>
            <div class="flex items-center gap-3 text-ui">
              <span class="flex items-center gap-1" data-testid="secret-set">
                <Icon name="lucide:lock" size="16" class="shrink-0" aria-hidden="true" />
                {{ labels.secretSet }}
              </span>
              <AppButton variant="ghost" size="xs" :disabled="busy" @click="replacingSecret = true">
                {{ labels.secretReplace }}
              </AppButton>
            </div>
          </div>
          <template v-else>
            <label class="flex flex-col gap-1">
              <span class="label text-ui text-base-content">{{ labels.clientSecret }}</span>
              <input
                :id="ids.secret"
                v-model="secret"
                type="password"
                class="input input-sm w-full font-mono text-ui"
                :class="{ 'input-error': fieldErrors.client_secret }"
                autocomplete="off"
                spellcheck="false"
                :disabled="busy"
                :aria-invalid="!!fieldErrors.client_secret"
                :aria-describedby="`${ids.secretHelp} ${ids.secretErr}`"
              >
            </label>
            <FieldError :id="ids.secretErr" :message="fieldErrors.client_secret" />
          </template>
          <p :id="ids.secretHelp" class="label whitespace-normal text-xs text-base-content/70">
            {{ labels.secretHelp }}
          </p>
        </div>
        <ErrorText :error="error" />
        <div class="flex flex-wrap items-center gap-3 pt-1">
          <AppButton type="submit" size="sm" :disabled="busy">
            {{ busy ? labels.saving : labels.save }}
          </AppButton>
          <AppButton v-if="!google" variant="ghost" size="sm" :disabled="busy" @click="emit('cancel')">
            {{ labels.cancel }}
          </AppButton>
          <SavedNote v-if="saved" :label="labels.saved" />
        </div>
      </form>
      <dl v-else-if="google" class="space-y-1 text-ui" data-testid="google-readonly">
        <dt class="label text-ui text-base-content">
          {{ labels.clientId }}
        </dt>
        <dd class="font-mono break-all">
          {{ google.clientId }}
        </dd>
        <dt class="label text-ui text-base-content">
          {{ labels.enabled }}
        </dt>
        <dd>{{ google.enabled ? labels.on : labels.off }}</dd>
      </dl>

      <div v-if="canEdit && google" class="space-y-3 border-t border-base-300 pt-3">
        <div class="space-y-1">
          <label :for="ids.enabled" class="flex items-center gap-3 text-ui font-medium">
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
          <p :id="ids.enabledHelp" class="label whitespace-normal text-xs text-base-content/70">
            {{ labels.enabledHelp }}
          </p>
          <p v-if="locked" :id="ids.locked" class="flex items-start gap-1 text-xs text-base-content">
            <Icon name="lucide:lock" size="14" class="shrink-0 text-info" aria-hidden="true" />
            {{ labels.locked }}
          </p>
          <ErrorText :error="toggleError" />
        </div>
        <div class="space-y-1">
          <AppButton variant="error" size="sm" soft :disabled="busy || locked || removeLocked" :aria-describedby="removeLocked ? ids.removeLocked : undefined" @click="emit('remove')">
            <Icon name="lucide:trash-2" size="16" class="shrink-0" aria-hidden="true" />
            {{ labels.remove }}
          </AppButton>
          <p v-if="removeLocked" :id="ids.removeLocked" class="flex items-start gap-1 text-xs text-base-content">
            <Icon name="lucide:lock" size="14" class="shrink-0 text-info" aria-hidden="true" />
            {{ google.enabled && !locked ? `${labels.removeLocked} ${labels.turnOffInstead}` : labels.removeLocked }}
          </p>
        </div>
      </div>
    </template>
  </Section>
</template>
