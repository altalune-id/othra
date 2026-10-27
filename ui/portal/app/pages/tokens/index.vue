<script setup lang="ts">
import TokenRow from '#base/components/account/TokenRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import { usePersonalTokens } from '~/components/portal/usePersonalTokens';

const { t } = useI18n();
const list = usePersonalTokens();
const { tokens, loading, loadError, revoke } = list;

useHead({ title: () => t('nav.tokens') });

await list.load();
</script>

<template>
  <div class="max-w-3xl space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold">
          {{ t('nav.tokens') }}
        </h1>
        <p class="text-sm text-base-content/70">
          {{ t('tokens.subtitle') }}
        </p>
      </div>
      <AppButton to="/tokens/new" size="sm">
        <Icon name="lucide:plus" size="16" />
        {{ t('tokens.new') }}
      </AppButton>
    </div>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <p v-else-if="!loadError && tokens.length === 0" class="card border border-base-300 bg-base-100 p-6 text-sm text-base-content/70">
      {{ t('tokens.empty') }}
    </p>

    <ul v-else-if="!loadError" class="list rounded-box border border-base-300 bg-base-100" data-testid="tokens">
      <TokenRow v-for="tok in tokens" :key="tok.id" :token="tok">
        <AppButton v-if="tok.status === 'active'" variant="ghost" size="sm" @click="revoke.ask(tok)">
          {{ t('common.revoke') }}
        </AppButton>
      </TokenRow>
    </ul>

    <ConfirmModal
      :handle="revoke"
      :title="t('tokens.revoke_title')"
      :body="t('tokens.revoke_body', { name: revoke.target.value?.name ?? '' })"
      :confirm-label="t('common.revoke')"
      :cancel-label="t('common.cancel')"
    />
  </div>
</template>
