<script setup lang="ts">
import AppAccess from '#base/components/account/AppAccess.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import { useConnectedApps } from '~/components/portal/useConnectedApps';

const { t } = useI18n();
const config = useRuntimeConfig().public;
const list = useConnectedApps();
const { apps, loading, loadError, disconnect } = list;
const target = computed(() => disconnect.target.value);

useHead({ title: () => t('nav.apps') });

await list.load();
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('nav.apps') }}
      </h1>
      <p class="text-sm text-base-content/70">
        {{ t('apps.subtitle', { project: config.project }) }}
      </p>
    </div>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <p v-else-if="!loadError && apps.length === 0" class="card border border-base-300 bg-base-100 p-6 text-sm text-base-content/70">
      {{ t('apps.empty') }}
    </p>

    <section v-for="app in apps" :key="app.clientId" class="card border border-base-300 bg-base-100" data-testid="app">
      <div class="card-body">
        <AppAccess :app="app" :signs-in="t(`apps.sees_${app.sees}`)">
          <AppButton variant="ghost" size="sm" class="shrink-0" @click="disconnect.ask(app)">
            {{ t('apps.disconnect') }}
          </AppButton>
        </AppAccess>
      </div>
    </section>

    <ConfirmModal
      :handle="disconnect"
      :title="t('apps.disconnect_title', { app: target?.name ?? '' })"
      :body="t(target?.consented ? 'apps.disconnect_body' : 'apps.disconnect_body_signin', { app: target?.name ?? '' })"
      :confirm-label="t('apps.disconnect')"
      :cancel-label="t('common.cancel')"
    />
  </div>
</template>
