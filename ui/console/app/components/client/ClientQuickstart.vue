<script setup lang="ts">
import type { QuickstartInput } from './quickstart';
import CopyButton from '#base/components/ui/CopyButton.vue';
import KeyValue from '#base/components/ui/KeyValue.vue';
import Section from '#base/components/ui/Section.vue';
import { quickstartEnv, tryAuthorizeUrl } from './quickstart';

const { values, name, openTo, backTo } = defineProps<{ values: QuickstartInput, name: string, openTo: string, backTo: string }>();

const { t } = useI18n();
const env = computed(() => Object.entries(quickstartEnv(values)).map(([k, v]) => `${k}=${v}`).join('\n'));
const items = computed(() => [
  { label: t('clients.issuer'), value: values.issuer, mono: true, copy: true },
  { label: t('clients.client_id'), value: values.clientId, mono: true, copy: true },
  { label: t('clients.redirect_uri'), value: values.redirectUri, mono: true, copy: true },
]);

async function trySignIn() {
  window.open(await tryAuthorizeUrl(values), '_blank', 'noopener');
}
</script>

<template>
  <Section :title="t('clients.quickstart_title', { name })" :description="t('clients.quickstart_body')" flush>
    <KeyValue :items="items" :copy-label="t('common.copy_label', { label: '{label}' })" />
    <div class="space-y-2 border-t border-base-300 p-4">
      <div class="flex items-center justify-between gap-2">
        <h3 class="text-sm font-medium">
          .env
        </h3>
        <span class="flex items-center gap-1 text-sm">
          {{ t('clients.copy_env') }}
          <CopyButton :value="env" :label="t('clients.copy_env')" />
        </span>
      </div>
      <pre class="overflow-x-auto rounded-box bg-base-200 p-3 font-mono text-xs" data-testid="quickstart-env">{{ env }}</pre>
      <p v-if="values.secret" class="text-xs text-base-content/70">
        {{ t('clients.quickstart_secret_note') }}
      </p>
    </div>
    <div class="flex flex-wrap gap-2">
      <button v-if="values.redirectUri" type="button" class="btn btn-primary btn-sm" @click="trySignIn">
        <Icon name="lucide:external-link" size="16" />
        {{ t('clients.try_signin') }}
      </button>
      <NuxtLink :to="openTo" class="btn btn-sm">
        {{ t('clients.open_client') }}
      </NuxtLink>
      <NuxtLink :to="backTo" class="btn btn-ghost btn-sm">
        {{ t('clients.back_to_clients') }}
      </NuxtLink>
    </div>
  </Section>
</template>
