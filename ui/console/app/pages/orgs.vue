<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SystemBadge from '#base/components/ui/SystemBadge.vue';
import { useOrgCreation } from '~/components/org/create/useOrgCreation';
import OrgsEmptyState from '~/components/org/OrgsEmptyState.vue';
import { siblingURL } from '~/lib/auth/urls';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useUIStore } from '~/stores/ui';

const { t } = useI18n();
const orgs = useOrgsStore();
const auth = useAuthStore();
const ui = useUIStore();
const { apiUrl } = useRuntimeConfig().public;

await Promise.all([orgs.ensureLoaded(), auth.load()]);

const { allowed, ownsOne, canCreate } = useOrgCreation();
const isEmpty = computed(() => !orgs.error && orgs.items.length === 0);

if (isEmpty.value && canCreate.value) {
  await navigateTo('/onboarding', { replace: true });
}
const emptyLabels = computed(() => ({
  title: t('org.none_title'),
  body: t('org.none_body'),
  create: t('org.none_create'),
  signOut: t('auth.sign_out'),
}));

useHead({ title: () => `${t('org.all')} · Othra` });
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-5 px-4 py-8 sm:px-6 sm:py-10">
    <PageHeader :title="t('org.title')" :description="isEmpty ? '' : t('org.subtitle')">
      <template #actions>
        <p v-if="allowed && ownsOne" class="max-w-56 text-right text-xs text-base-content/70" data-testid="org-create-owned">
          {{ t('org.create_owned') }}
        </p>
        <AppButton v-else-if="canCreate && !isEmpty" size="sm" @click="ui.openOrgCreate()">
          <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
          {{ t('org.new') }}
        </AppButton>
      </template>
    </PageHeader>

    <LoadError v-if="orgs.error" :error="orgs.error" :busy="orgs.loading" :retry-label="t('common.retry')" @retry="orgs.load()" />
    <OrgsEmptyState
      v-else-if="isEmpty"
      :can-create="canCreate"
      :logout-url="siblingURL(apiUrl, '/logout')"
      :labels="emptyLabels"
      @create="ui.openOrgCreate()"
    />
    <ul v-else class="divide-y divide-base-300 rounded-box border border-base-300 bg-base-100" :aria-label="t('org.title')">
      <li v-for="org in orgs.items" :key="org.slug" class="flex items-center gap-2 px-4 py-2 text-ui">
        <NuxtLink :to="`/o/${org.slug}`" class="link link-hover min-w-0 truncate font-medium">
          {{ org.name }}
        </NuxtLink>
        <span class="min-w-0 truncate font-mono text-xs text-base-content/70">{{ org.slug }}</span>
        <SystemBadge v-if="org.system" :label="t('common.system')" class="ms-auto shrink-0" />
      </li>
    </ul>
  </div>
</template>
