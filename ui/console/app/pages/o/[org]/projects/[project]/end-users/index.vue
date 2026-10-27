<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import LoadingStatus from '#base/components/ui/LoadingStatus.vue';
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
const searchId = useId();

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('endusers.title')} · Othra` });

await list.load();
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('endusers.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('endusers.subtitle') }}
      </p>
    </div>
    <form role="search" class="flex gap-2" @submit.prevent="list.load()">
      <label :for="searchId" class="sr-only">{{ t('endusers.search') }}</label>
      <input :id="searchId" v-model="query" type="search" class="input w-full max-w-sm" :placeholder="t('endusers.search')" autocomplete="off">
      <AppButton type="submit" variant="plain" size="sm" class="self-center">
        {{ t('endusers.search_action') }}
      </AppButton>
    </form>
    <LoadError :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="list.load()" />
    <p v-if="!loading && users.length === 0 && !loadError" class="text-sm text-base-content/60">
      {{ shown ? t('endusers.none_found') : t('endusers.empty') }}
    </p>
    <div v-else-if="!loadError" class="overflow-x-auto rounded-box border border-base-300">
      <table class="table" data-testid="end-users">
        <thead>
          <tr>
            <th>{{ t('endusers.email') }}</th>
            <th>{{ t('common.name') }}</th>
            <th class="max-sm:hidden">
              {{ t('endusers.last_sign_in') }}
            </th>
            <th class="max-sm:hidden">
              {{ t('endusers.created') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id" class="row-hover">
            <td class="break-all">
              <NuxtLink :to="`${base}/${u.id}`" class="link font-medium">
                {{ u.email }}
              </NuxtLink>
            </td>
            <td>{{ u.name || '—' }}</td>
            <td class="max-sm:hidden">
              {{ date(u.lastSignInAt) }}
            </td>
            <td class="max-sm:hidden">
              {{ date(u.createdAt) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <LoadingStatus v-if="loading" :label="t('common.loading')" />
    <AppButton v-else-if="next" variant="plain" size="sm" @click="list.more()">
      {{ t('endusers.more') }}
    </AppButton>
  </div>
</template>
