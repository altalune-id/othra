<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import type { TokenExpiry, TokenServer } from '#gen/account/v1/account_pb';
import { useId } from 'vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import { expiryOptions } from './useTokenCreate';

const { servers, server, busy = false, error = null, fieldErrors, reauthHref = '' } = defineProps<{
  servers: TokenServer[],
  server: TokenServer | null,
  busy?: boolean,
  error?: ShownError | null,
  fieldErrors: Record<string, string>,
  reauthHref?: string,
}>();

const emit = defineEmits<{ submit: [] }>();
const name = defineModel<string>('name', { required: true });
const serverId = defineModel<string>('serverId', { required: true });
const scopes = defineModel<string[]>('scopes', { required: true });
const expiry = defineModel<TokenExpiry>('expiry', { required: true });
const { t } = useI18n();
const ids = { name: useId(), nameErr: useId(), server: useId(), serverHelp: useId(), serverErr: useId(), scopesHelp: useId(), scopesErr: useId(), expires: useId() };
</script>

<template>
  <form novalidate class="card border border-base-300 bg-base-100" @submit.prevent="emit('submit')">
    <div class="card-body gap-2">
      <div class="fieldset">
        <label :for="ids.name" class="fieldset-legend">{{ t('common.name') }}</label>
        <input :id="ids.name" v-model="name" class="input w-full" :class="{ 'input-error': fieldErrors.name }" :disabled="busy" :aria-invalid="!!fieldErrors.name" :aria-describedby="ids.nameErr">
        <FieldError :id="ids.nameErr" :message="fieldErrors.name" />
      </div>
      <div class="fieldset">
        <label :for="ids.server" class="fieldset-legend">{{ t('tokens.server') }}</label>
        <select :id="ids.server" v-model="serverId" class="select w-full" :class="{ 'select-error': fieldErrors.server_id }" :disabled="busy" :aria-describedby="`${ids.serverHelp} ${ids.serverErr}`">
          <option value="" disabled>
            {{ t('tokens.pick_server') }}
          </option>
          <option v-for="s in servers" :key="s.id" :value="s.id">
            {{ s.name || s.identifier }}
          </option>
        </select>
        <FieldError :id="ids.serverErr" :message="fieldErrors.server_id" />
        <p :id="ids.serverHelp" class="label whitespace-normal">
          {{ t('tokens.server_help') }}
        </p>
      </div>
      <fieldset v-if="server" class="fieldset" :aria-describedby="`${ids.scopesHelp} ${ids.scopesErr}`">
        <legend class="fieldset-legend">
          {{ t('tokens.access') }}
        </legend>
        <label v-for="s in server.scopes" :key="s" class="flex items-start gap-2 text-sm">
          <input v-model="scopes" type="checkbox" :value="s" class="checkbox checkbox-sm mt-0.5 rounded-sm" :disabled="busy">
          <span>
            <span class="block">{{ server.scopeDescriptions[s] || s }}</span>
            <span class="block font-mono text-xs text-base-content/70">{{ s }}</span>
          </span>
        </label>
        <FieldError :id="ids.scopesErr" :message="fieldErrors.scopes" />
        <p :id="ids.scopesHelp" class="label whitespace-normal">
          {{ t('tokens.access_help') }}
        </p>
      </fieldset>
      <div class="fieldset">
        <label :for="ids.expires" class="fieldset-legend">{{ t('tokens.expires') }}</label>
        <select :id="ids.expires" v-model.number="expiry" class="select w-full" :disabled="busy">
          <option v-for="o in expiryOptions" :key="o.value" :value="o.value">
            {{ t('tokens.days', { days: o.days }) }}
          </option>
        </select>
      </div>
      <ErrorText :error="error" />
      <AppButton v-if="reauthHref" variant="plain" size="sm" class="self-start" :href="reauthHref">
        {{ t('tokens.reauth') }}
      </AppButton>
      <div class="flex items-center gap-3 pt-1">
        <AppButton type="submit" size="sm" :disabled="busy">
          {{ busy ? t('common.creating') : t('common.create') }}
        </AppButton>
        <AppButton variant="ghost" size="sm" to="/tokens">
          {{ t('common.cancel') }}
        </AppButton>
      </div>
    </div>
  </form>
</template>
