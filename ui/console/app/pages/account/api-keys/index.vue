<script setup lang="ts">
import type { PersonalKeyView } from '~/components/apikey/personal/PersonalKeys.vue';
import AppButton from '#base/components/ui/AppButton.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { useDates } from '#base/composables/useDates';
import { expiryState, relativeTime } from '#base/lib/tokens/keyStatus';
import PersonalKeys from '~/components/apikey/personal/PersonalKeys.vue';
import { usePersonalKeys } from '~/components/apikey/personal/usePersonalKeys';
import { loginURL } from '~/lib/auth/urls';

const { t, locale } = useI18n();
const { date } = useDates();
const personal = usePersonalKeys();
const { rows, failed, oneOrg, signedIn, error, created } = personal;
const loginUrl = computed(() => (import.meta.client ? loginURL(useRuntimeConfig().public.apiUrl, window.location) : ''));

const now = new Date();
const keys = computed<PersonalKeyView[]>(() => {
  return rows.value.map(k => ({
    rowKey: k.rowKey,
    orgId: k.orgId,
    orgName: k.orgName,
    name: k.name,
    secretHint: k.secretHint,
    scopes: k.scopeLabels,
    lastUsed: relativeTime(k.lastUsedAt, now, locale.value),
    expires: date(k.expiresAt),
    expiry: expiryState(k.expiresAt, now),
    revoked: !!k.revokedAt,
  }));
});

const labels = computed(() => ({
  name: t('common.name'),
  org: t('apikeys.col_org'),
  scopes: t('apikeys.col_scopes'),
  lastUsed: t('apikeys.col_last_used'),
  neverUsed: t('tokens.never_used'),
  expires: t('apikeys.expires'),
  expiresSoon: (days: number) => t('tokens.expires_soon', { days }),
  expired: t('tokens.status_expired'),
  status: t('apikeys.col_status'),
  active: t('tokens.status_active'),
  revoked: t('tokens.status_revoked'),
  revoke: t('common.revoke'),
  revokeConfirm: t('apikeys.revoke_confirm'),
  revokeBody: (name: string, hint: string) => t('apikeys.revoke_body', { name, hint }),
  actions: t('apikeys.col_actions'),
  actionsFor: (name: string) => t('apikeys.actions_for', { name }),
  caption: t('apikeys.personal_title'),
  cancel: t('common.cancel'),
  empty: t('apikeys.personal_empty'),
  newKey: t('apikeys.personal_new'),
  loadFailed: (org: string) => t('apikeys.org_load_failed', { org }),
  retry: t('common.retry'),
  secretWarning: t('apikeys.secret_warning'),
  copy: t('apikeys.copy'),
  snippet: t('apikeys.mcp_snippet'),
  copySnippet: t('apikeys.copy_snippet'),
  done: t('apikeys.done'),
  signIn: t('apikeys.personal_sign_in'),
  signInLink: t('auth.sign_in'),
}));

onBeforeUnmount(() => personal.dismiss());

await personal.load();

useHead({ title: () => `${t('apikeys.personal_title')} · Othra` });
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('apikeys.personal_title')" :description="t('apikeys.personal_subtitle')">
      <template v-if="signedIn && keys.length > 0" #actions>
        <AppButton size="sm" to="/account/api-keys/new">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('apikeys.personal_new') }}
        </AppButton>
      </template>
    </PageHeader>
    <PersonalKeys
      :keys="keys"
      :one-org="oneOrg"
      :failed="failed"
      :created="created"
      :error="error"
      :signed-in="signedIn"
      :login-url="loginUrl"
      :labels="labels"
      @revoke="personal.revoke"
      @retry="personal.retry"
      @dismiss="personal.dismiss"
    />
  </div>
</template>
