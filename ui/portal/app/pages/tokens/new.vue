<script setup lang="ts">
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import TokenForm from '~/components/portal/TokenForm.vue';
import TokenReveal from '~/components/portal/TokenReveal.vue';
import { useTokenCreate } from '~/components/portal/useTokenCreate';
import { reauthURL } from '~/lib/hosted';

const { t } = useI18n();
const form = useTokenCreate();
const { servers, server, loading, loadError, name, serverId, scopes, expiry, busy, error, fieldErrors, secret, snippet, reauth } = form;
const config = useRuntimeConfig().public;
const route = useRoute();
const router = useRouter();
const reauthHref = computed(() => (reauth.value ? reauthURL(config.hostedUrl, router.resolve(route.fullPath).href) : ''));

async function done() {
  form.forget();
  await navigateTo('/tokens');
}

onBeforeUnmount(() => form.forget());

useHead({ title: () => t('tokens.new') });

await form.load();
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <NuxtLink to="/tokens" class="link text-sm">
        ← {{ t('nav.tokens') }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold">
        {{ t('tokens.new') }}
      </h1>
    </div>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="form.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <TokenReveal v-else-if="secret" :secret="secret" :snippet="snippet" @done="done()" />
    <p v-else-if="!loadError && servers.length === 0" class="card border border-base-300 bg-base-100 p-6 text-sm text-base-content/70">
      {{ t('tokens.no_servers', { project: config.project }) }}
    </p>
    <TokenForm
      v-else-if="!loadError"
      v-model:name="name"
      v-model:server-id="serverId"
      v-model:scopes="scopes"
      v-model:expiry="expiry"
      :servers="servers"
      :server="server"
      :busy="busy"
      :error="error"
      :field-errors="fieldErrors"
      :reauth-href="reauthHref"
      @submit="form.submit()"
    />
  </div>
</template>
