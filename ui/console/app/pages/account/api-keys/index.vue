<script setup lang="ts">
import type { PersonalKeyView } from '~/components/apikey/personal/PersonalKeys.vue';
import { expiryState, relativeTime } from '~/components/apikey/personal/keyStatus';
import PersonalKeys from '~/components/apikey/personal/PersonalKeys.vue';
import { usePersonalKeys } from '~/components/apikey/personal/usePersonalKeys';
import { loginURL } from '~/lib/auth/urls';

const { t, locale } = useI18n();
const personal = usePersonalKeys();
const { rows, failed, oneOrg, signedIn, error, created } = personal;
const loginUrl = computed(() => (import.meta.client ? loginURL(useRuntimeConfig().public.apiUrl, window.location) : ''));

const now = new Date();
const keys = computed<PersonalKeyView[]>(() => {
  const date = new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' });
  return rows.value.map(k => ({
    rowKey: k.rowKey,
    orgId: k.orgId,
    orgName: k.orgName,
    name: k.name,
    secretHint: k.secretHint,
    scopes: k.scopeLabels,
    lastUsed: relativeTime(k.lastUsedAt, now, locale.value),
    expires: k.expiresAt ? date.format(k.expiresAt) : '',
    expiry: expiryState(k.expiresAt, now),
    revoked: !!k.revokedAt,
  }));
});

const labels = computed(() => ({
  name: t('common.name'),
  org: t('apikeys.col_org'),
  scopes: t('apikeys.col_scopes'),
  lastUsed: t('apikeys.col_last_used'),
  neverUsed: t('apikeys.never_used'),
  expires: t('apikeys.expires'),
  expiresSoon: (days: number) => t('apikeys.expires_soon', { days }),
  expired: t('apikeys.expired'),
  status: t('apikeys.col_status'),
  active: t('apikeys.active'),
  revoked: t('apikeys.revoked'),
  revoke: t('common.revoke'),
  revokeConfirm: t('apikeys.revoke_confirm'),
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
  <div class="space-y-6">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold">
          {{ t('apikeys.personal_title') }}
        </h1>
        <p class="text-sm text-base-content/60">
          {{ t('apikeys.personal_subtitle') }}
        </p>
      </div>
      <NuxtLink v-if="signedIn && keys.length > 0" to="/account/api-keys/new" class="btn btn-primary btn-sm shrink-0">
        <Icon name="lucide:plus" size="16" />
        {{ t('apikeys.personal_new') }}
      </NuxtLink>
    </div>
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
