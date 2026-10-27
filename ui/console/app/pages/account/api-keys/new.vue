<script setup lang="ts">
import PersonalKeyCreateForm from '~/components/apikey/personal/PersonalKeyCreateForm.vue';
import { usePersonalKeyCreate } from '~/components/apikey/personal/usePersonalKeyCreate';

const { t } = useI18n();
const form = usePersonalKeyCreate();
const { name, orgId, orgName, orgOptions, oneOrg, expiresInDays, allProjects, projectIds, scopes, projects, mintable, busy, error, canSubmit } = form;

const expiryOptions = computed(() => form.expiryOptions.map(o => ({ days: o.days, label: t(o.labelKey) })));
const labels = computed(() => ({
  name: t('common.name'),
  org: t('apikeys.org_field'),
  orgHelp: t('apikeys.org_help'),
  selectOrg: t('apikeys.select_org'),
  expires: t('apikeys.expires'),
  expiresHelp: t('apikeys.expires_help'),
  projects: t('apikeys.projects'),
  allProjects: t('apikeys.all_projects'),
  selectProjects: t('apikeys.select_projects'),
  scopes: t('apikeys.scopes'),
  scopesHelp: t('apikeys.scopes_help'),
  scopesPickOrg: t('apikeys.scopes_pick_org'),
  cancel: t('common.cancel'),
  create: t('common.create'),
  creating: t('common.creating'),
}));

await form.load();

useHead({ title: () => `${t('apikeys.personal_new')} · ${t('apikeys.personal_title')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <NuxtLink to="/account/api-keys" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
        <Icon name="lucide:chevron-left" size="14" />
        {{ t('apikeys.personal_title') }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold">
        {{ t('apikeys.personal_new') }}
      </h1>
    </div>
    <PersonalKeyCreateForm
      v-model:name="name"
      v-model:org-id="orgId"
      v-model:expires-in-days="expiresInDays"
      v-model:all-projects="allProjects"
      v-model:project-ids="projectIds"
      v-model:scopes="scopes"
      :org-options="orgOptions"
      :one-org="oneOrg"
      :org-name="orgName"
      :expiry-options="expiryOptions"
      :projects="projects"
      :scope-options="mintable"
      :busy="busy"
      :can-submit="canSubmit"
      :error="error"
      :labels="labels"
      @select-org="form.selectOrg"
      @submit="form.submit"
    />
  </div>
</template>
