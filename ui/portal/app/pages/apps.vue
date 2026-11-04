<script setup lang="ts">
import AppAccess from '#base/components/account/AppAccess.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
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
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('nav.apps')" :description="t('apps.subtitle', { project: config.project })" />
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <EmptyState v-else-if="!loadError && apps.length === 0" icon="lucide:blocks" :title="t('apps.empty')" />

    <section v-for="app in apps" :key="app.clientId" class="rounded-box border border-base-300 bg-base-100 p-4" data-testid="app">
      <AppAccess :app="app" :signs-in="t(`apps.sees_${app.sees}`)">
        <AppButton variant="ghost" size="sm" class="shrink-0" @click="disconnect.ask(app)">
          <Icon name="lucide:unplug" size="16" class="shrink-0 text-error" aria-hidden="true" />
          {{ t('apps.disconnect') }}
        </AppButton>
      </AppAccess>
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
