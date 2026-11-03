<script setup lang="ts">
import TokenRow from '#base/components/account/TokenRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { usePersonalTokens } from '~/components/portal/usePersonalTokens';

const { t } = useI18n();
const list = usePersonalTokens();
const { tokens, loading, loadError, revoke } = list;

useHead({ title: () => t('nav.tokens') });

await list.load();
</script>

<template>
  <div class="flex max-w-3xl flex-col gap-5">
    <PageHeader :title="t('nav.tokens')" :description="t('tokens.subtitle')">
      <template #actions>
        <AppButton to="/tokens/new" size="sm">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('tokens.new') }}
        </AppButton>
      </template>
    </PageHeader>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <EmptyState v-else-if="!loadError && tokens.length === 0" icon="lucide:key-round" :title="t('tokens.empty')" />

    <ul v-else-if="!loadError" class="list rounded-box border border-base-300 bg-base-100" data-testid="tokens">
      <TokenRow v-for="tok in tokens" :key="tok.id" :token="tok">
        <AppButton v-if="tok.status === 'active'" variant="ghost" size="sm" @click="revoke.ask(tok)">
          <Icon name="lucide:ban" size="16" class="shrink-0 text-error" aria-hidden="true" />
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
