<script setup lang="ts">
import MethodRow from '#base/components/account/MethodRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { useLinkedNotice } from '~/components/portal/useLinkedNotice';
import { useSignInMethods } from '~/components/portal/useSignInMethods';
import { useAccountStore } from '~/stores/account';

const { t } = useI18n();
const config = useRuntimeConfig().public;
const methods = useSignInMethods();
const account = useAccountStore();
const { rows, linkToken, loading, loadError, unlinked, unlink } = methods;
const linked = useLinkedNotice(unlinked);

useHead({ title: () => t('nav.sign_in') });

await methods.load();
</script>

<template>
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('nav.sign_in')" :description="t('methods.subtitle', { project: config.project })" />
    <div v-if="linked || unlinked" role="status" class="alert items-start text-ui">
      <Icon name="lucide:circle-check" size="16" class="shrink-0 text-success" aria-hidden="true" />
      <span>{{ linked ? t('methods.connected') : t('methods.disconnected') }}</span>
    </div>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="methods.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />

    <ul v-else-if="rows.length > 0" class="list rounded-box border border-base-300 bg-base-100" data-testid="methods">
      <MethodRow v-for="row in rows" :key="row.key" :row="row" :account-email="account.profile?.email ?? ''">
        <template v-if="row.key === 'google'">
          <AppButton v-if="row.linked" variant="ghost" size="sm" :aria-disabled="!row.canRemove" @click="methods.askUnlink(row)">
            <Icon name="lucide:unplug" size="16" class="shrink-0 text-error" aria-hidden="true" />
            {{ t('methods.disconnect') }}
          </AppButton>
          <form v-else method="post" :action="`${config.hostedUrl}/federation/google/start`">
            <input type="hidden" name="csrf" :value="linkToken">
            <input type="hidden" name="intent" value="link">
            <AppButton type="submit" variant="plain" size="sm">
              {{ t('methods.connect') }}
            </AppButton>
          </form>
        </template>
      </MethodRow>
    </ul>

    <ConfirmModal :handle="unlink" :title="t('methods.disconnect_title')" :body="t('methods.disconnect_body')" :confirm-label="t('methods.disconnect')" :cancel-label="t('common.cancel')" />
  </div>
</template>
