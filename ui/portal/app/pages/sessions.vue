<script setup lang="ts">
import SessionRow from '#base/components/account/SessionRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { useSessions } from '~/components/portal/useSessions';

const { t } = useI18n();
const config = useRuntimeConfig().public;
const list = useSessions();
const { sessions, others, loading, loadError, end, endOthers } = list;

useHead({ title: () => t('nav.sessions') });

await list.load();
</script>

<template>
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('nav.sessions')" :description="t('sessions.subtitle', { project: config.project })" />
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <LoadingStatus v-if="loading" :label="t('common.loading')" />

    <section v-else-if="!loadError" class="rounded-box border border-base-300 bg-base-100">
      <ul class="list" data-testid="sessions">
        <SessionRow v-for="s in sessions" :key="s.id" :session="s">
          <AppButton v-if="!s.current" variant="ghost" size="sm" @click="end.ask(s)">
            <Icon name="lucide:log-out" size="16" class="shrink-0 text-error" aria-hidden="true" />
            {{ t('sessions.sign_out') }}
          </AppButton>
        </SessionRow>
      </ul>
      <div v-if="others.length > 0" class="border-t border-base-300 px-4 py-3">
        <AppButton variant="error" soft size="sm" @click="endOthers.ask(true)">
          <Icon name="lucide:log-out" size="16" class="shrink-0 text-error" aria-hidden="true" />
          {{ t('sessions.sign_out_others') }}
        </AppButton>
      </div>
    </section>

    <ConfirmModal :handle="end" :title="t('sessions.sign_out_title')" :body="t('sessions.sign_out_body')" :confirm-label="t('sessions.sign_out')" :cancel-label="t('common.cancel')" />
    <ConfirmModal :handle="endOthers" :title="t('sessions.others_title')" :body="t('sessions.others_body')" :confirm-label="t('sessions.sign_out_others')" :cancel-label="t('common.cancel')" />
  </div>
</template>
