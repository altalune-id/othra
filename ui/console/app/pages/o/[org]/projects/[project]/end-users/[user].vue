<script setup lang="ts">
import AppAccess from '#base/components/account/AppAccess.vue';
import MethodRow from '#base/components/account/MethodRow.vue';
import SessionRow from '#base/components/account/SessionRow.vue';
import TokenRow from '#base/components/account/TokenRow.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import ConfirmModal from '#base/components/ui/ConfirmModal.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import LoadError from '#base/components/ui/LoadError.vue';
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
const back = computed(() => `/o/${route.params.org}/projects/${route.params.project}/end-users`);

useHead({ title: () => `${user.value?.email ?? t('endusers.title')} · Othra` });

await Promise.all([detail.load(), auth.load()]);
if (notFound.value) {
  throw createError({ statusCode: 404, statusMessage: t('apperror.EUS001'), fatal: true });
}
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div>
      <NuxtLink :to="back" class="link text-sm">
        ← {{ t('endusers.title') }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold break-all">
        {{ user?.email }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ canRevoke ? t('endusers.detail_subtitle_revoke') : t('endusers.detail_subtitle_read') }}
      </p>
    </div>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="detail.load()" />

    <template v-if="ready && user">
      <section class="card border border-base-300 bg-base-100">
        <dl class="card-body grid gap-3 sm:grid-cols-2">
          <div>
            <dt class="fieldset-legend">
              {{ t('common.name') }}
            </dt>
            <dd>{{ user.name || '—' }}</dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('endusers.sub') }}
            </dt>
            <dd class="flex items-center gap-1 font-mono text-sm break-all">
              {{ user.id }}
              <CopyButton :value="user.id" :label="t('common.copy_label', { label: t('endusers.sub') })" />
            </dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('endusers.created') }}
            </dt>
            <dd>{{ date(user.createdAt) }}</dd>
          </div>
          <div>
            <dt class="fieldset-legend">
              {{ t('endusers.last_sign_in') }}
            </dt>
            <dd>{{ date(user.lastSignInAt) }}</dd>
          </div>
        </dl>
      </section>

      <section class="card border border-base-300 bg-base-100">
        <div class="card-body gap-2">
          <h2 class="card-title text-base">
            {{ t('endusers.methods') }}
          </h2>
          <p class="text-xs text-base-content/60">
            {{ t('endusers.methods_help') }}
          </p>
          <ul class="list" data-testid="end-user-methods">
            <MethodRow v-for="m in methods" :key="m.key" :row="m" :account-email="user.email" />
          </ul>
        </div>
      </section>

      <section class="card border border-base-300 bg-base-100">
        <div class="card-body gap-2">
          <h2 class="card-title text-base">
            {{ t('endusers.sessions') }}
          </h2>
          <p v-if="sessions.length === 0" class="text-sm text-base-content/60">
            {{ t('endusers.sessions_empty') }}
          </p>
          <ul v-else class="list" data-testid="end-user-sessions">
            <SessionRow v-for="s in sessions" :key="s.id" :session="s">
              <AppButton v-if="canRevoke" variant="ghost" size="sm" @click="endSession.ask(s)">
                {{ t('endusers.sign_out') }}
              </AppButton>
            </SessionRow>
          </ul>
        </div>
      </section>

      <section class="card border border-base-300 bg-base-100">
        <div class="card-body gap-2">
          <h2 class="card-title text-base">
            {{ t('endusers.apps') }}
          </h2>
          <p v-if="apps.length === 0" class="text-sm text-base-content/60">
            {{ t('endusers.apps_empty') }}
          </p>
          <ul v-else class="list" data-testid="end-user-apps">
            <li v-for="app in apps" :key="app.clientId" class="list-row">
              <AppAccess :app="app" :signs-in="t('endusers.signs_in')" class="list-col-grow">
                <AppButton v-if="canRevoke" variant="ghost" size="sm" class="shrink-0" @click="disconnect.ask(app)">
                  {{ t('apps.disconnect') }}
                </AppButton>
              </AppAccess>
            </li>
          </ul>
        </div>
      </section>

      <section class="card border border-base-300 bg-base-100">
        <div class="card-body gap-2">
          <h2 class="card-title text-base">
            {{ t('endusers.tokens') }}
          </h2>
          <p v-if="tokens.length === 0" class="text-sm text-base-content/60">
            {{ t('endusers.tokens_empty') }}
          </p>
          <ul v-else class="list" data-testid="end-user-tokens">
            <TokenRow v-for="tok in tokens" :key="tok.id" :token="tok">
              <AppButton v-if="canRevoke && tok.status === 'active'" variant="ghost" size="sm" @click="revokeToken.ask(tok)">
                {{ t('common.revoke') }}
              </AppButton>
            </TokenRow>
          </ul>
        </div>
      </section>
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
