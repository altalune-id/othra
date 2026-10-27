<script setup lang="ts">
import ClientForm from '~/components/client/ClientForm.vue';
import ClientSecretDialog from '~/components/client/ClientSecretDialog.vue';
import { clientFormLabels, kindOptions, secretDialogLabels } from '~/components/client/labels';
import { useClientForm } from '~/components/client/useClientForm';
import { useCanEditProject } from '~/components/project/useProjectAccess';
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
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/clients`);
if (!canEdit.value) {
  await navigateTo(base.value, { replace: true });
}

const form = useClientForm(projectId);
const { fields, warnings, uriErrors, nameError, busy, error, revealed } = form;
const labels = computed(() => clientFormLabels(t, 'create'));
const kinds = computed(() => kindOptions(t));
const dialogLabels = computed(() => secretDialogLabels(t));

async function submit() {
  const client = await form.submit();
  if (!client) {
    return;
  }
  if (detail.get(projectId)) {
    void detail.load(projectId);
  }
  if (!revealed.value) {
    await navigateTo(`${base.value}/${client.id}`);
  }
}

async function secretSaved() {
  const id = revealed.value?.clientId ?? '';
  form.forget();
  await navigateTo({ path: `${base.value}/${id}`, query: { focus: 'secrets' } });
}

function warnLeave(event: BeforeUnloadEvent) {
  if (revealed.value) {
    event.preventDefault();
  }
}

onBeforeRouteLeave(() => !revealed.value);
onMounted(() => window.addEventListener('beforeunload', warnLeave));
onBeforeUnmount(() => window.removeEventListener('beforeunload', warnLeave));

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('clients.new')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <NuxtLink :to="base" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
        <Icon name="lucide:chevron-left" size="14" />
        {{ t('clients.title') }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold">
        {{ t('clients.new') }}
      </h1>
    </div>
    <section class="card border border-base-300 bg-base-100">
      <div class="card-body">
        <ClientForm
          v-model:fields="fields"
          mode="create"
          :kinds="kinds"
          :uri-errors="uriErrors"
          :warnings="warnings"
          :name-error="nameError"
          :busy="busy"
          :error="error"
          :labels="labels"
          @kind="form.setKind"
          @edit="form.clearRow"
          @remove="form.removeRow"
          @submit="submit"
          @cancel="navigateTo(base)"
        />
      </div>
    </section>
    <ClientSecretDialog
      :open="!!revealed"
      :owner-id="revealed?.clientId ?? ''"
      :secret="revealed?.secret ?? ''"
      :labels="dialogLabels"
      @done="secretSaved"
    />
  </div>
</template>
