<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import type { ClientFields } from './clientForm';
import type { ClientKindName } from './kinds';
import type { RedirectUriListLabels } from './RedirectUriList.vue';
import { computed, useId } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import ClientKindPicker from './ClientKindPicker.vue';
import RedirectUriList from './RedirectUriList.vue';

export interface ClientKindOption {
  value: ClientKindName;
  icon: string;
  label: string;
  help: string;
  rec: string;
  redirectHelp: string;
  placeholder: string;
  postLogout: boolean;
  secret: boolean;
}

export interface ClientFormLabels {
  kind: string;
  kindFixed: string;
  name: string;
  nameHelp: string;
  redirectUris: string;
  postLogoutUris: string;
  postLogoutHelp: string;
  refresh: string;
  refreshHelp: string;
  consent: string;
  consentRequire: string;
  consentImplicit: string;
  consentImplicitWarning: string;
  authMethod: string;
  authMethodBasic: string;
  authMethodPost: string;
  authMethodHelp: string;
  pkce: string;
  pkceRequired: string;
  pkceOffWarning: string;
  submit: string;
  submitting: string;
  cancel: string;
  back: string;
  uriList: RedirectUriListLabels;
  postLogoutList: RedirectUriListLabels;
}

const props = withDefaults(defineProps<{
  kinds: ClientKindOption[],
  mode?: 'create' | 'edit',
  uriErrors?: { redirect: string[], postLogout: string[] },
  warnings?: { redirect: boolean[], postLogout: boolean[] },
  nameError?: string,
  busy?: boolean,
  error?: ShownError | null,
  labels?: ClientFormLabels,
  kindChosen?: boolean,
}>(), {
  mode: 'create',
  uriErrors: () => ({ redirect: [], postLogout: [] }),
  warnings: () => ({ redirect: [], postLogout: [] }),
  nameError: '',
  busy: false,
  error: null,
  labels: () => ({
    kind: 'Kind',
    kindFixed: 'The kind can\'t change later.',
    name: 'Name',
    nameHelp: 'Shown to end users on the sign-in and consent pages.',
    redirectUris: 'Redirect URIs',
    postLogoutUris: 'Sign-out return URIs',
    postLogoutHelp: 'Optional. Where end users land after signing out of the app.',
    refresh: 'Refresh tokens',
    refreshHelp: 'Lets the app stay signed in without asking the end user again.',
    consent: 'Consent',
    consentRequire: 'Ask the end user',
    consentImplicit: 'Skip consent (first-party app)',
    consentImplicitWarning: 'Only for your own apps. End users won\'t see what this app can access.',
    authMethod: 'How the app sends its secret',
    authMethodBasic: 'Authorization header (client_secret_basic)',
    authMethodPost: 'Request body (client_secret_post)',
    authMethodHelp: 'Use the header unless the app\'s library only supports the body.',
    pkce: 'PKCE',
    pkceRequired: 'Require PKCE (recommended)',
    pkceOffWarning: 'Only for apps that can\'t send PKCE, such as a conformance test client. The app must then send a nonce on every sign-in.',
    submit: 'Create client',
    submitting: 'Creating…',
    cancel: 'Cancel',
    back: 'Back',
    uriList: { uri: 'Redirect URI', remove: 'Remove redirect URI', add: 'Add URI', localhostWarning: '127.0.0.1 is safer than localhost, which some systems resolve elsewhere.' },
    postLogoutList: { uri: 'Sign-out return URI', remove: 'Remove sign-out return URI', add: 'Add URI', localhostWarning: '127.0.0.1 is safer than localhost, which some systems resolve elsewhere.' },
  }),
});

const emit = defineEmits<{
  submit: [],
  cancel: [],
  back: [],
  kind: [kind: ClientKindName],
  edit: [list: 'redirect' | 'postLogout', index: number],
  remove: [list: 'redirect' | 'postLogout', index: number],
}>();
const fields = defineModel<ClientFields>('fields', { required: true });
const ids = { name: useId(), nameHelp: useId(), nameErr: useId(), consentWarn: useId(), authHelp: useId(), pkceWarn: useId() };

const kind = computed(() => props.kinds.find(k => k.value === fields.value.kind) ?? props.kinds[0]);

function patch(next: Partial<ClientFields>) {
  fields.value = { ...fields.value, ...next };
}
</script>

<template>
  <form novalidate class="space-y-2" @submit.prevent="emit('submit')">
    <ClientKindPicker
      v-if="props.mode === 'create' && !props.kindChosen"
      :kinds="props.kinds"
      :model-value="fields.kind"
      :busy="busy"
      :legend="labels.kind"
      :note="labels.kindFixed"
      @update:model-value="emit('kind', $event)"
    />

    <div class="fieldset">
      <label class="flex flex-col gap-1">
        <span class="label text-ui text-base-content">{{ labels.name }}</span>
        <input
          :id="ids.name"
          :value="fields.name"
          type="text"
          class="input input-sm w-full text-ui"
          :class="{ 'input-error': nameError }"
          maxlength="100"
          :disabled="busy"
          :aria-invalid="!!nameError"
          :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`"
          @input="patch({ name: ($event.target as HTMLInputElement).value })"
        >
      </label>
      <FieldError :id="ids.nameErr" :message="nameError" />
      <p :id="ids.nameHelp" class="label whitespace-normal text-xs text-base-content/70">
        {{ labels.nameHelp }}
      </p>
    </div>

    <RedirectUriList
      :rows="fields.redirectUris"
      :label="labels.redirectUris"
      :help="kind?.redirectHelp"
      :placeholder="kind?.placeholder"
      :errors="uriErrors.redirect"
      :warnings="warnings.redirect"
      :disabled="busy"
      :labels="labels.uriList"
      @update:rows="patch({ redirectUris: $event })"
      @edit="emit('edit', 'redirect', $event)"
      @remove="emit('remove', 'redirect', $event)"
    />

    <RedirectUriList
      v-if="kind?.postLogout"
      :rows="fields.postLogoutUris"
      :label="labels.postLogoutUris"
      :help="labels.postLogoutHelp"
      :errors="uriErrors.postLogout"
      :warnings="warnings.postLogout"
      :required="false"
      :disabled="busy"
      :labels="labels.postLogoutList"
      @update:rows="patch({ postLogoutUris: $event })"
      @edit="emit('edit', 'postLogout', $event)"
      @remove="emit('remove', 'postLogout', $event)"
    />

    <fieldset class="fieldset">
      <legend class="fieldset-legend text-ui">
        {{ labels.refresh }}
      </legend>
      <label class="flex items-center gap-2 text-ui">
        <input
          type="checkbox"
          class="checkbox checkbox-sm rounded-sm"
          :checked="fields.refreshTokens"
          :disabled="busy"
          @change="patch({ refreshTokens: ($event.target as HTMLInputElement).checked })"
        >
        {{ labels.refreshHelp }}
      </label>
    </fieldset>

    <template v-if="kind?.secret">
      <fieldset class="fieldset" :aria-describedby="ids.authHelp">
        <legend class="fieldset-legend text-ui">
          {{ labels.authMethod }}
        </legend>
        <label class="flex items-center gap-2 text-ui">
          <input type="radio" name="client-auth-method" class="radio radio-sm" :checked="fields.authMethod === 'basic'" :disabled="busy" @change="patch({ authMethod: 'basic' })">
          {{ labels.authMethodBasic }}
        </label>
        <label class="flex items-center gap-2 text-ui">
          <input type="radio" name="client-auth-method" class="radio radio-sm" :checked="fields.authMethod === 'post'" :disabled="busy" @change="patch({ authMethod: 'post' })">
          {{ labels.authMethodPost }}
        </label>
        <p :id="ids.authHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ labels.authMethodHelp }}
        </p>
      </fieldset>

      <fieldset class="fieldset">
        <legend class="fieldset-legend text-ui">
          {{ labels.pkce }}
        </legend>
        <label class="flex items-center gap-2 text-ui">
          <input
            type="checkbox"
            class="checkbox checkbox-sm rounded-sm"
            :checked="fields.pkceRequired"
            :disabled="busy"
            :aria-describedby="fields.pkceRequired ? undefined : ids.pkceWarn"
            @change="patch({ pkceRequired: ($event.target as HTMLInputElement).checked })"
          >
          {{ labels.pkceRequired }}
        </label>
        <p v-if="!fields.pkceRequired" :id="ids.pkceWarn" class="mt-1 flex items-start gap-1 text-xs text-base-content">
          <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" aria-hidden="true" />
          {{ labels.pkceOffWarning }}
        </p>
      </fieldset>
    </template>

    <fieldset class="fieldset">
      <legend class="fieldset-legend text-ui">
        {{ labels.consent }}
      </legend>
      <label class="flex items-center gap-2 text-ui">
        <input type="radio" name="client-consent" class="radio radio-sm" :checked="fields.consent === 'require'" :disabled="busy" @change="patch({ consent: 'require' })">
        {{ labels.consentRequire }}
      </label>
      <label class="flex items-center gap-2 text-ui">
        <input type="radio" name="client-consent" class="radio radio-sm" :checked="fields.consent === 'implicit'" :disabled="busy" :aria-describedby="ids.consentWarn" @change="patch({ consent: 'implicit' })">
        {{ labels.consentImplicit }}
      </label>
      <p v-if="fields.consent === 'implicit'" :id="ids.consentWarn" class="mt-1 flex items-start gap-1 text-xs text-base-content">
        <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" aria-hidden="true" />
        {{ labels.consentImplicitWarning }}
      </p>
    </fieldset>

    <ErrorText :error="error" />
    <div v-if="props.mode === 'create'" class="flex gap-2 pt-2">
      <AppButton variant="plain" size="sm" :disabled="busy" @click="emit('back')">
        {{ labels.back }}
      </AppButton>
      <AppButton type="submit" size="sm" :busy="busy">
        {{ busy ? labels.submitting : labels.submit }}
      </AppButton>
    </div>
    <div v-else class="flex items-center gap-3 pt-2">
      <AppButton type="submit" size="sm" :busy="busy">
        {{ busy ? labels.submitting : labels.submit }}
      </AppButton>
    </div>
  </form>
</template>
