<script setup lang="ts">
import type { ShownError } from '#base/lib/apperror/message';
import AppButton from '#base/components/ui/AppButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import KeyValue from '#base/components/ui/KeyValue.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import ProjectEnvironmentCard from '~/components/project/ProjectEnvironmentCard.vue';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const ids = { slugHelp: useId() };
const errorMessage = useErrorMessage();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const { $projectClient: client } = useNuxtApp();
const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() =>
  projects.items.find(item => item.orgId === org.value?.id && item.slug === route.params.project),
);
const guard = useProjectGuard(() => project.value);
const canEdit = computed(() => guard.value.canConfigure);
await useAuthStore().load();

if (!project.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const name = ref(project.value.name);
const slug = ref(project.value.slug);
const busy = ref(false);
const error = ref<ShownError | null>(null);
const saved = ref(false);
watch([name, slug], () => {
  saved.value = false;
});

async function save() {
  const current = project.value;
  if (!current || busy.value) {
    return;
  }
  busy.value = true;
  error.value = null;
  saved.value = false;
  try {
    const res = await client.updateProject({ id: current.id, name: name.value.trim(), slug: slug.value.trim() });
    if (res.project) {
      projects.update(res.project.id, { name: res.project.name, slug: res.project.slug });
      if (res.project.slug !== route.params.project) {
        await navigateTo(`/o/${route.params.org}/projects/${res.project.slug}/settings`, { replace: true });
      }
    }
    saved.value = true;
  } catch (e) {
    error.value = errorMessage(e, 'error.rename_project');
  } finally {
    busy.value = false;
  }
}

useHead({ title: () => `${project.value?.name ?? t('project.title')} · ${t('project.settings_general')} · Othra` });

const facts = computed(() => {
  const id = { label: t('common.id'), value: project.value?.id ?? '', mono: true, copy: true };
  if (canEdit.value) {
    return [id];
  }
  return [id, { label: t('common.name'), value: project.value?.name ?? '' }, { label: t('project.console_slug'), value: project.value?.slug ?? '', mono: true }];
});
</script>

<template>
  <div v-if="project" class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('project.settings_general')" :description="t('project.settings_general_subtitle')" />
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="`${t('project.system_note')} ${t('project.system_general')}`" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <div class="rounded-box border border-base-300 bg-base-100" :data-testid="canEdit ? undefined : 'general-readonly'">
      <KeyValue :items="facts" :copy-label="t('common.copy_label', { label: '{label}' })" />
    </div>

    <form v-if="canEdit" novalidate class="flex flex-col gap-4" @submit.prevent="save()">
      <label class="flex flex-col gap-1">
        <span class="label text-ui text-base-content">{{ t('common.name') }}</span>
        <input v-model="name" type="text" class="input input-sm w-full text-ui" :disabled="busy">
      </label>
      <div class="flex flex-col gap-1">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('project.console_slug') }}</span>
          <input v-model="slug" type="text" class="input input-sm w-full font-mono text-ui" :aria-describedby="ids.slugHelp" :disabled="busy">
        </label>
        <p :id="ids.slugHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('project.console_slug_help') }}
        </p>
      </div>
      <ErrorText :error="error" />
      <div class="flex items-center gap-3">
        <AppButton type="submit" size="sm" :disabled="busy || !name.trim()">
          <Icon name="lucide:check" size="16" class="shrink-0" aria-hidden="true" />
          {{ busy ? t('common.saving') : t('common.save') }}
        </AppButton>
        <SavedNote v-if="saved" :label="t('common.saved')" />
      </div>
    </form>
    <p v-else class="text-xs text-base-content/70">
      {{ t('project.console_slug_help') }}
    </p>

    <ProjectEnvironmentCard v-if="!guard.system" :project-id="project.id" :environment="project.environment" :can-edit="canEdit" />
  </div>
</template>
