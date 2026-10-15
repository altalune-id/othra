<script setup lang="ts">
import CopyButton from '~/components/ui/CopyButton.vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() =>
  projects.items.find(item => item.orgId === org.value?.id && item.slug === route.params.project),
);

if (!project.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}`);

const sections = computed(() => [
  { key: 'webhooks', to: `${base.value}/webhooks`, icon: 'lucide:webhook', title: t('webhooks.title'), hint: t('webhooks.subtitle') },
  { key: 'settings', to: `${base.value}/settings`, icon: 'lucide:settings', title: t('nav.settings'), hint: t('project.settings_hint') },
]);

useHead({ title: () => `${project.value?.name ?? t('project.title')} · Othra` });
</script>

<template>
  <div v-if="project" class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ project.name }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('nav.overview') }}
      </p>
      <div class="mt-1 flex items-center gap-1 text-xs text-base-content/60">
        <span class="font-mono">{{ project.id }}</span>
        <CopyButton :value="project.id" :label="t('common.copy')" />
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <NuxtLink
        v-for="section in sections"
        :key="section.key"
        :to="section.to"
        class="card border border-base-300 bg-base-100 transition-colors hover:border-primary"
      >
        <div class="card-body">
          <h2 class="card-title text-base">
            <Icon :name="section.icon" size="18" />
            {{ section.title }}
          </h2>
          <p class="text-sm text-base-content/60">
            {{ section.hint }}
          </p>
        </div>
      </NuxtLink>
    </div>
  </div>
</template>
