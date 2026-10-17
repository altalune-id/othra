<script setup lang="ts">
import type { ShownError } from '~/lib/apperror/message';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import CopyButton from '~/components/ui/CopyButton.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import SavedNote from '~/components/ui/SavedNote.vue';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const ids = { name: useId(), slug: useId(), slugHelp: useId() };
const errorMessage = useErrorMessage();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const { $projectClient: client } = useNuxtApp();
const canEdit = useCanEditProject();
await useAuthStore().load();

const org = computed(() => orgs.bySlug(route.params.org as string));
const project = computed(() =>
  projects.items.find(item => item.orgId === org.value?.id && item.slug === route.params.project),
);

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
</script>

<template>
  <div v-if="project" class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('project.settings_general') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('project.settings_general_subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" />

    <section class="card border border-base-300 bg-base-100">
      <div class="card-body gap-2">
        <div class="fieldset">
          <h2 class="fieldset-legend">
            {{ t('common.id') }}
          </h2>
          <div class="flex items-center gap-2 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
            <code class="min-w-0 flex-1 font-mono text-sm break-all">{{ project.id }}</code>
            <CopyButton :value="project.id" :label="t('common.copy')" class="shrink-0" />
          </div>
        </div>

        <form v-if="canEdit" novalidate class="space-y-2" @submit.prevent="save()">
          <div class="fieldset">
            <label :for="ids.name" class="fieldset-legend">{{ t('common.name') }}</label>
            <input :id="ids.name" v-model="name" type="text" class="input w-full" :disabled="busy">
          </div>
          <div class="fieldset">
            <label :for="ids.slug" class="fieldset-legend">{{ t('project.console_slug') }}</label>
            <input :id="ids.slug" v-model="slug" type="text" class="input w-full font-mono" :aria-describedby="ids.slugHelp" :disabled="busy">
            <p :id="ids.slugHelp" class="label whitespace-normal">
              {{ t('project.console_slug_help') }}
            </p>
          </div>
          <ErrorText :error="error" />
          <div class="flex items-center gap-3 pt-2">
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !name.trim()">
              {{ busy ? t('common.saving') : t('common.save') }}
            </button>
            <SavedNote v-if="saved" :label="t('common.saved')" />
          </div>
        </form>
        <dl v-else class="space-y-2" data-testid="general-readonly">
          <div class="fieldset">
            <dt class="fieldset-legend">
              {{ t('common.name') }}
            </dt>
            <dd>{{ project.name }}</dd>
          </div>
          <div class="fieldset">
            <dt class="fieldset-legend">
              {{ t('project.console_slug') }}
            </dt>
            <dd class="font-mono text-sm">
              {{ project.slug }}
            </dd>
            <dd class="label whitespace-normal">
              {{ t('project.console_slug_help') }}
            </dd>
          </div>
        </dl>
      </div>
    </section>
  </div>
</template>
