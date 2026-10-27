<script setup lang="ts">
import type { ClientRow } from '~/components/client/ClientList.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import { useDates } from '#base/composables/useDates';
import ClientList from '~/components/client/ClientList.vue';
import { clientKinds, isSystemClient, kindName } from '~/components/client/kinds';
import { useClientList } from '~/components/client/useClients';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const { date } = useDates();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
const guard = useProjectGuard(() => listed.value);
const canEdit = computed(() => guard.value.canConfigure);
await useAuthStore().load();
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/clients`);
const list = useClientList(listed.value.id);
const { loading, loadError } = list;
const rows = computed<ClientRow[]>(() => {
  return list.clients.value.map(c => ({
    id: c.id,
    name: c.name,
    kind: t(clientKinds[kindName(c.kind)].labelKey),
    created: date(c.createdAt),
    to: `${base.value}/${c.id}`,
    system: isSystemClient(c),
  }));
});
const labels = computed(() => ({
  name: t('common.name'),
  clientId: t('clients.client_id'),
  kind: t('clients.kind'),
  created: t('clients.col_created'),
  copyFor: t('clients.copy_id_for', { name: '{name}' }),
  system: t('common.system'),
}));

onMounted(() => list.load());

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('clients.title')} · Othra` });
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold" tabindex="-1">
          {{ t('clients.title') }}
        </h1>
        <p class="text-sm text-base-content/60">
          {{ t('clients.subtitle') }}
        </p>
      </div>
      <NuxtLink v-if="canEdit" :to="`${base}/new`" class="btn btn-primary btn-sm max-sm:btn-block">
        <Icon name="lucide:plus" size="16" />
        {{ t('clients.new') }}
      </NuxtLink>
    </div>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('clients.system_note')" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <section class="card border border-base-300 bg-base-100" data-testid="clients-card">
      <div class="card-body gap-2">
        <template v-if="loadError">
          <ErrorText :error="loadError" />
          <div>
            <button type="button" class="btn btn-sm" @click="list.load()">
              {{ t('common.retry') }}
            </button>
          </div>
        </template>
        <div v-else-if="!loading && rows.length === 0" class="flex flex-col items-center gap-2 py-6 text-center">
          <Icon name="lucide:app-window" size="28" class="text-base-content/60" />
          <p class="font-medium">
            {{ t('clients.empty_title') }}
          </p>
          <p class="max-w-md text-sm text-base-content/70">
            {{ t('clients.empty_body') }}
          </p>
          <NuxtLink v-if="canEdit" :to="`${base}/new`" class="btn btn-primary btn-sm mt-2">
            <Icon name="lucide:plus" size="16" />
            {{ t('clients.new') }}
          </NuxtLink>
        </div>
        <ClientList v-else :rows="rows" :loading="loading" :labels="labels" />
      </div>
    </section>
  </div>
</template>
