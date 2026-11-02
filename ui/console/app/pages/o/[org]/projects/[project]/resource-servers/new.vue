<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { scopeTableLabels } from '~/components/resourceserver/labels';
import ScopeTable from '~/components/resourceserver/ScopeTable.vue';
import { useResourceServerCreate } from '~/components/resourceserver/useResourceServers';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const detail = useProjectDetailStore();
const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
const guard = useProjectGuard(() => listed.value);
const canEdit = computed(() => guard.value.canConfigure);
await useAuthStore().load();

if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}
const projectId = listed.value.id;
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/resource-servers`);
if (!canEdit.value) {
  await navigateTo(base.value, { replace: true });
}

const form = useResourceServerCreate(projectId);
const { name, identifier, scopes, busy, error, errors, trustAvailable, trustPicked, trustRequired, trusted } = form;
const scopeRows = scopes.rows;
const scopeDescriptions = scopes.descriptions;
const scopeErrors = scopes.errors;
const scopeLabels = computed(() => scopeTableLabels(t));
const ids = { name: useId(), nameHelp: useId(), nameErr: useId(), uri: useId(), uriHelp: useId(), uriErr: useId(), trust: useId() };

async function submit() {
  const rs = await form.submit();
  if (!rs) {
    return;
  }
  if (detail.get(projectId)) {
    void detail.load(projectId);
  }
  await navigateTo({ path: `${base.value}/${rs.id}`, query: { verify: '1' } });
}

onMounted(() => form.loadTrust());

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('rs.new')} · Othra` });
</script>

<template>
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('rs.new')" :description="t('rs.subtitle')" />
    <form novalidate class="flex flex-col gap-4" @submit.prevent="submit">
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('rs.uri') }}</span>
          <input
            :id="ids.uri"
            v-model="identifier"
            type="url"
            class="input input-sm w-full font-mono text-ui"
            :class="{ 'input-error': errors.identifier }"
            placeholder="https://api.example.com"
            maxlength="512"
            autocomplete="off"
            spellcheck="false"
            :disabled="busy"
            :aria-invalid="!!errors.identifier"
            :aria-describedby="`${ids.uriHelp} ${ids.uriErr}`"
            @input="form.clearIdentifier()"
          >
        </label>
        <FieldError :id="ids.uriErr" :message="errors.identifier" />
        <p :id="ids.uriHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('rs.uri_help') }}
        </p>
      </div>
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('common.name') }}</span>
          <input
            :id="ids.name"
            v-model="name"
            type="text"
            class="input input-sm w-full text-ui"
            :class="{ 'input-error': errors.name }"
            maxlength="100"
            :disabled="busy"
            :aria-invalid="!!errors.name"
            :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`"
          >
        </label>
        <FieldError :id="ids.nameErr" :message="errors.name" />
        <p :id="ids.nameHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('rs.name_help') }}
        </p>
      </div>
      <fieldset v-if="trustAvailable" class="fieldset" data-testid="trust-on-create">
        <legend class="fieldset-legend text-ui">
          {{ t('rs.trust') }}
        </legend>
        <label class="flex items-center gap-2 text-ui">
          <input
            type="checkbox"
            class="checkbox checkbox-xs rounded-sm"
            :checked="trusted"
            :disabled="busy || trustRequired"
            :aria-describedby="ids.trust"
            @change="trustPicked = ($event.target as HTMLInputElement).checked"
          >
          {{ t('rs.trust_create') }}
        </label>
        <p v-if="trusted" :id="ids.trust" class="mt-1 flex items-start gap-1 text-xs text-base-content">
          <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" aria-hidden="true" />
          {{ trustRequired ? t('rs.trust_required') : t('rs.trust_body') }}
        </p>
        <p v-else :id="ids.trust" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('rs.trust_help') }}
        </p>
      </fieldset>
      <ScopeTable
        v-model:rows="scopeRows"
        v-model:descriptions="scopeDescriptions"
        :errors="scopeErrors"
        :disabled="busy"
        :full="scopes.full.value"
        :labels="scopeLabels"
        @add="scopes.add()"
        @remove="scopes.remove"
        @edit="scopes.clearRow"
      />
      <ErrorText :error="error" />
      <div class="flex justify-end gap-2 pt-2 max-sm:flex-col-reverse">
        <AppButton variant="ghost" size="sm" :to="base">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton type="submit" size="sm" :busy="busy">
          {{ busy ? t('common.creating') : t('rs.create') }}
        </AppButton>
      </div>
    </form>
  </div>
</template>
