<script setup lang="ts">
import { useCanEditProject } from '~/components/project/useProjectAccess';
import { scopeTableLabels } from '~/components/resourceserver/labels';
import ScopeTable from '~/components/resourceserver/ScopeTable.vue';
import { useResourceServerCreate } from '~/components/resourceserver/useResourceServers';
import ErrorText from '~/components/ui/ErrorText.vue';
import FieldError from '~/components/ui/FieldError.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const detail = useProjectDetailStore();
const canEdit = useCanEditProject();
await useAuthStore().load();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
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
  <div class="max-w-2xl space-y-6">
    <div>
      <NuxtLink :to="base" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
        <Icon name="lucide:chevron-left" size="14" />
        {{ t('rs.title') }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold">
        {{ t('rs.new') }}
      </h1>
    </div>
    <section class="card border border-base-300 bg-base-100">
      <div class="card-body">
        <form novalidate class="space-y-2" @submit.prevent="submit">
          <div class="fieldset">
            <label :for="ids.uri" class="fieldset-legend">{{ t('rs.uri') }}</label>
            <input
              :id="ids.uri"
              v-model="identifier"
              type="url"
              class="input w-full font-mono text-sm"
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
            <FieldError :id="ids.uriErr" :message="errors.identifier" />
            <p :id="ids.uriHelp" class="label whitespace-normal">
              {{ t('rs.uri_help') }}
            </p>
          </div>
          <div class="fieldset">
            <label :for="ids.name" class="fieldset-legend">{{ t('common.name') }}</label>
            <input
              :id="ids.name"
              v-model="name"
              type="text"
              class="input w-full"
              :class="{ 'input-error': errors.name }"
              maxlength="100"
              :disabled="busy"
              :aria-invalid="!!errors.name"
              :aria-describedby="`${ids.nameHelp} ${ids.nameErr}`"
            >
            <FieldError :id="ids.nameErr" :message="errors.name" />
            <p :id="ids.nameHelp" class="label whitespace-normal">
              {{ t('rs.name_help') }}
            </p>
          </div>
          <fieldset v-if="trustAvailable" class="fieldset" data-testid="trust-on-create">
            <legend class="fieldset-legend">
              {{ t('rs.trust') }}
            </legend>
            <label class="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                class="checkbox checkbox-sm rounded-sm"
                :checked="trusted"
                :disabled="busy || trustRequired"
                :aria-describedby="ids.trust"
                @change="trustPicked = ($event.target as HTMLInputElement).checked"
              >
              {{ t('rs.trust_create') }}
            </label>
            <p v-if="trusted" :id="ids.trust" class="mt-1 flex items-start gap-1 text-xs text-base-content">
              <Icon name="lucide:triangle-alert" size="14" class="shrink-0 text-warning" />
              {{ trustRequired ? t('rs.trust_required') : t('rs.trust_body') }}
            </p>
            <p v-else :id="ids.trust" class="label whitespace-normal">
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
            <NuxtLink :to="base" class="btn btn-ghost btn-sm">
              {{ t('common.cancel') }}
            </NuxtLink>
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
              <span v-if="busy" class="loading loading-spinner loading-xs" />
              {{ busy ? t('common.creating') : t('rs.create') }}
            </button>
          </div>
        </form>
      </div>
    </section>
  </div>
</template>
