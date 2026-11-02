<script setup lang="ts">
import type { Column } from '#base/components/ui/types';
import AppButton from '#base/components/ui/AppButton.vue';
import DataTable from '#base/components/ui/DataTable.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SearchInput from '#base/components/ui/SearchInput.vue';
import { useDates } from '#base/composables/useDates';
import { useEndUserList } from '~/components/enduser/useEndUsers';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const list = useEndUserList(listed.value.id);
const { users, query, shown, next, loading, loadError } = list;
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/end-users`);
const { date } = useDates('—');
const columns = computed<Column[]>(() => [
  { key: 'email', label: t('endusers.email') },
  { key: 'name', label: t('common.name') },
  { key: 'lastSignInAt', label: t('endusers.last_sign_in') },
  { key: 'createdAt', label: t('endusers.created') },
]);

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('endusers.title')} · Othra` });

void list.load();
</script>

<template>
  <div class="flex max-w-4xl flex-col gap-4">
    <PageHeader :title="t('endusers.title')" :description="t('endusers.subtitle')" />
    <form role="search" class="flex items-center gap-2" @submit.prevent="list.load()">
      <SearchInput v-model="query" :label="t('endusers.search')" class="w-full sm:w-64" />
      <AppButton type="submit" variant="plain" size="sm">
        {{ t('endusers.search_action') }}
      </AppButton>
    </form>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <EmptyState v-if="!loading && users.length === 0 && !loadError" :icon="shown ? 'lucide:search-x' : 'lucide:users'" :title="shown ? t('endusers.none_found') : t('endusers.empty')" />
    <DataTable v-else-if="!loadError && users.length" :caption="t('endusers.title')" row-key="id" :columns="columns" :rows="users" data-testid="end-users">
      <template #cell-email="{ row }">
        <NuxtLink :to="`${base}/${row.id}`" class="link link-hover font-medium break-all">
          {{ row.email }}
        </NuxtLink>
      </template>
      <template #cell-name="{ row }">
        {{ row.name || '—' }}
      </template>
      <template #cell-lastSignInAt="{ row }">
        <span class="whitespace-nowrap tabular-nums text-base-content/70">{{ date(row.lastSignInAt) }}</span>
      </template>
      <template #cell-createdAt="{ row }">
        <span class="whitespace-nowrap tabular-nums text-base-content/70">{{ date(row.createdAt) }}</span>
      </template>
    </DataTable>
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <AppButton v-else-if="next" variant="plain" size="sm" class="self-start" @click="list.more()">
      {{ t('endusers.more') }}
    </AppButton>
  </div>
</template>
