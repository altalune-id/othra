<script setup lang="ts">
import AppAccess from '#base/components/account/AppAccess.vue';
import MethodRow from '#base/components/account/MethodRow.vue';
import SessionRow from '#base/components/account/SessionRow.vue';
import TokenRow from '#base/components/account/TokenRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import KeyValue from '#base/components/ui/KeyValue.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import Section from '#base/components/ui/Section.vue';
import { useDates } from '#base/composables/useDates';
import { useEndUser } from '~/components/enduser/useEndUser';
import { useCan } from '~/composables/useCan';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const auth = useAuthStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const detail = useEndUser(listed.value.id, route.params.user as string);
const { user, methods, sessions, apps, tokens, notFound, loading, loadError, ready, endSession, disconnect, revokeToken } = detail;
const canRevoke = useCan('end_user:revoke');
const { date } = useDates('—');
const facts = computed(() => [
  { label: t('common.name'), value: user.value?.name || '—' },
  { label: t('endusers.sub'), value: user.value?.id ?? '', mono: true, copy: true },
  { label: t('endusers.created'), value: date(user.value?.createdAt) },
  { label: t('endusers.last_sign_in'), value: date(user.value?.lastSignInAt) },
]);

useHead({ title: () => `${user.value?.email ?? t('endusers.title')} · Othra` });

await Promise.all([detail.load(), auth.load()]);
if (notFound.value) {
  throw createError({ statusCode: 404, statusMessage: t('apperror.EUS001'), fatal: true });
}
</script>

<template>
  <div class="flex max-w-4xl flex-col gap-5">
    <PageHeader :title="user?.email ?? ''" :description="canRevoke ? t('endusers.detail_subtitle_revoke') : t('endusers.detail_subtitle_read')" />
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="detail.load()" />

    <template v-if="ready && user">
      <div class="rounded-box border border-base-300 bg-base-100">
        <KeyValue :items="facts" :copy-label="t('common.copy_label', { label: '{label}' })" />
      </div>

      <Section :title="t('endusers.methods')" :description="t('endusers.methods_help')" flush>
        <ul class="list" data-testid="end-user-methods">
          <MethodRow v-for="m in methods" :key="m.key" :row="m" :account-email="user.email" />
        </ul>
      </Section>

      <Section :title="t('endusers.sessions')" flush>
        <p v-if="sessions.length === 0" class="px-4 py-3 text-ui text-base-content/70">
          {{ t('endusers.sessions_empty') }}
        </p>
        <ul v-else class="list" data-testid="end-user-sessions">
          <SessionRow v-for="s in sessions" :key="s.id" :session="s">
            <AppButton v-if="canRevoke" variant="ghost" size="sm" @click="endSession.ask(s)">
              <Icon name="lucide:log-out" size="16" class="shrink-0 text-error" aria-hidden="true" />
              {{ t('endusers.sign_out') }}
            </AppButton>
          </SessionRow>
        </ul>
      </Section>

      <Section :title="t('endusers.apps')" flush>
        <p v-if="apps.length === 0" class="px-4 py-3 text-ui text-base-content/70">
          {{ t('endusers.apps_empty') }}
        </p>
        <ul v-else class="list" data-testid="end-user-apps">
          <li v-for="app in apps" :key="app.clientId" class="list-row">
            <AppAccess :app="app" :signs-in="t('endusers.signs_in')" class="list-col-grow">
              <AppButton v-if="canRevoke" variant="ghost" size="sm" class="shrink-0" @click="disconnect.ask(app)">
                <Icon name="lucide:unplug" size="16" class="shrink-0 text-error" aria-hidden="true" />
                {{ t('apps.disconnect') }}
              </AppButton>
            </AppAccess>
          </li>
        </ul>
      </Section>

      <Section :title="t('endusers.tokens')" flush>
        <p v-if="tokens.length === 0" class="px-4 py-3 text-ui text-base-content/70">
          {{ t('endusers.tokens_empty') }}
        </p>
        <ul v-else class="list" data-testid="end-user-tokens">
          <TokenRow v-for="tok in tokens" :key="tok.id" :token="tok">
            <AppButton v-if="canRevoke && tok.status === 'active'" variant="ghost" size="sm" @click="revokeToken.ask(tok)">
              <Icon name="lucide:ban" size="16" class="shrink-0 text-error" aria-hidden="true" />
              {{ t('common.revoke') }}
            </AppButton>
          </TokenRow>
        </ul>
      </Section>
    </template>

    <ConfirmModal :handle="endSession" :title="t('sessions.sign_out_title')" :body="t('endusers.sign_out_body')" :confirm-label="t('endusers.sign_out')" :cancel-label="t('common.cancel')" />
    <ConfirmModal
      :handle="disconnect"
      :title="t('apps.disconnect_title', { app: disconnect.target.value?.name ?? '' })"
      :body="t('endusers.disconnect_body')"
      :confirm-label="t('apps.disconnect')"
      :cancel-label="t('common.cancel')"
    />
    <ConfirmModal
      :handle="revokeToken"
      :title="t('tokens.revoke_title')"
      :body="t('tokens.revoke_body', { name: revokeToken.target.value?.name ?? '' })"
      :confirm-label="t('common.revoke')"
      :cancel-label="t('common.cancel')"
    />
  </div>
</template>
