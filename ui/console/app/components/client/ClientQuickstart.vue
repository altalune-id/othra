<script setup lang="ts">
import type { QuickstartInput } from './quickstart';
import AppButton from '#base/components/ui/AppButton.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import { authorizeLines, goLines, quickstartEnv, tryAuthorizeUrl } from './quickstart';

const { values, openTo, backTo, secret = false } = defineProps<{ values: QuickstartInput, openTo: string, backTo: string, secret?: boolean }>();

const { t } = useI18n();
const envLines = computed(() => Object.entries(quickstartEnv(values, secret)).map(([k, v]) => `${k}=${v}`));
const env = computed(() => envLines.value.join('\n'));
const tabs = computed(() => [
  { key: 'env', label: '.env', lines: envLines.value },
  { key: 'authorize', label: t('clients.tab_authorize'), lines: authorizeLines(values) },
  { key: 'go', label: 'Go', lines: goLines(values, secret) },
]);
const group = `quickstart-${useId()}`;

async function trySignIn() {
  window.open(await tryAuthorizeUrl(values), '_blank', 'noopener');
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-4">
    <div class="tabs tabs-border tabs-sm min-w-0" role="group" :aria-label="t('clients.quickstart_snippets')">
      <template v-for="(tab, i) in tabs" :key="tab.key">
        <label class="tab text-base-content!">
          <input type="radio" :name="group" :checked="i === 0">
          {{ tab.label }}
        </label>
        <div class="tab-content min-w-0 pt-3">
          <div class="mockup-code w-full min-w-0 text-xs" :data-testid="`quickstart-${tab.key}`">
            <pre v-for="(line, n) in tab.lines" :key="n" :data-prefix="n + 1"><code>{{ line }}</code></pre>
          </div>
          <div v-if="tab.key === 'env'" class="flex flex-col gap-2 pt-3">
            <CopyButton :value="env" :label="t('clients.copy_env')" :text="t('clients.copy_env')" class="self-start" />
            <p v-if="secret" class="text-xs text-base-content/70">
              {{ t('clients.quickstart_secret_note') }}
            </p>
          </div>
        </div>
      </template>
    </div>
    <p v-if="values.redirectUri && values.issuer" class="text-xs text-base-content/70">
      {{ t('clients.try_signin_note') }}
    </p>
    <div class="flex flex-wrap gap-2">
      <AppButton v-if="values.redirectUri && values.issuer" size="sm" @click="trySignIn">
        <Icon name="lucide:external-link" size="16" class="shrink-0" aria-hidden="true" />
        {{ t('clients.try_signin') }}
      </AppButton>
      <AppButton variant="plain" size="sm" :to="openTo">
        {{ t('clients.open_client') }}
      </AppButton>
      <AppButton variant="ghost" size="sm" :to="backTo">
        {{ t('clients.back_to_clients') }}
      </AppButton>
    </div>
  </div>
</template>
