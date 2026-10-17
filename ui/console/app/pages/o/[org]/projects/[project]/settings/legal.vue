<script setup lang="ts">
import { useLegal } from '~/components/project/legal/useLegal';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import AppModal from '~/components/ui/AppModal.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import FieldError from '~/components/ui/FieldError.vue';
import SavedNote from '~/components/ui/SavedNote.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t, locale } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const legal = useLegal(listed.value.id);
const { termsUrl, privacyUrl, hasTerms, version, nextVersion, publishedAt, urlChangedHint, busy, saved, clearing, bumping, bumpedTo, error, fieldErrors } = legal;
const ids = { terms: useId(), termsHint: useId(), termsErr: useId(), privacy: useId(), privacyHelp: useId(), privacyErr: useId() };
const published = computed(() => (publishedAt.value ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }).format(publishedAt.value) : ''));

const canEdit = useCanEditProject();
await Promise.all([legal.load(), useAuthStore().load()]);

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('legal.title')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('legal.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('legal.subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" />

    <form v-if="canEdit" novalidate class="card border border-base-300 bg-base-100" @submit.prevent="legal.save()">
      <div class="card-body gap-2">
        <div class="fieldset">
          <label :for="ids.terms" class="fieldset-legend">{{ t('legal.terms_url') }}</label>
          <input :id="ids.terms" v-model="termsUrl" type="url" class="input w-full" placeholder="https://example.com/terms" :aria-describedby="urlChangedHint ? `${ids.termsHint} ${ids.termsErr}` : ids.termsErr" :disabled="busy">
          <FieldError :id="ids.termsErr" :message="fieldErrors.terms_url" />
          <p v-if="urlChangedHint" :id="ids.termsHint" class="flex items-start gap-1 text-sm text-base-content/70">
            <Icon name="lucide:info" size="14" class="mt-0.5 shrink-0 text-warning" />
            {{ t('legal.url_changed_hint') }}
          </p>
        </div>
        <div class="fieldset">
          <label :for="ids.privacy" class="fieldset-legend">{{ t('legal.privacy_url') }}</label>
          <input :id="ids.privacy" v-model="privacyUrl" type="url" class="input w-full" placeholder="https://example.com/privacy" :aria-describedby="`${ids.privacyHelp} ${ids.privacyErr}`" :disabled="busy">
          <FieldError :id="ids.privacyErr" :message="fieldErrors.privacy_url" />
          <p :id="ids.privacyHelp" class="label whitespace-normal">
            {{ t('legal.privacy_help') }}
          </p>
        </div>
        <ErrorText v-if="!bumping" :error="error" />
        <div class="flex items-center gap-3 pt-1">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
            {{ busy ? t('common.saving') : t('common.save') }}
          </button>
          <SavedNote v-if="saved" :label="t('common.saved')" />
        </div>
      </div>
    </form>
    <dl v-else class="card border border-base-300 bg-base-100" data-testid="legal-readonly">
      <div class="card-body gap-4">
        <div>
          <dt class="fieldset-legend">
            {{ t('legal.terms_url') }}
          </dt>
          <dd class="break-all text-sm">
            <template v-if="termsUrl">
              {{ termsUrl }}
            </template>
            <span v-else class="text-base-content/50">—</span>
          </dd>
        </div>
        <div>
          <dt class="fieldset-legend">
            {{ t('legal.privacy_url') }}
          </dt>
          <dd class="break-all text-sm">
            <template v-if="privacyUrl">
              {{ privacyUrl }}
            </template>
            <span v-else class="text-base-content/50">—</span>
          </dd>
        </div>
      </div>
    </dl>

    <section class="card border border-base-300 bg-base-100">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('legal.acceptance_title') }}
        </h2>
        <template v-if="hasTerms">
          <p class="font-medium" data-testid="terms-version">
            {{ t('legal.current_version', { version, date: published }) }}
          </p>
          <p class="text-sm text-base-content/60">
            {{ t('legal.acceptance_body') }}
          </p>
          <SavedNote v-if="bumpedTo" :label="t('legal.bump_done', { version: bumpedTo })" />
          <div v-if="canEdit" class="flex justify-end">
            <button type="button" class="btn btn-outline btn-sm" :disabled="busy" @click="legal.askBump()">
              <Icon name="lucide:triangle-alert" size="16" class="text-warning" />
              {{ t('legal.bump') }}
            </button>
          </div>
        </template>
        <p v-else class="text-sm text-base-content/60">
          {{ canEdit ? t('legal.no_terms') : t('legal.no_terms_readonly') }}
        </p>
      </div>
    </section>

    <AppModal :open="clearing" :title="t('legal.clear_terms_title')" @close="legal.cancelClear()">
      <p class="text-sm">
        {{ t('legal.clear_terms_body') }}
      </p>
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" @click="legal.cancelClear()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-warning btn-sm" :disabled="busy" @click="legal.confirmClear()">
          {{ t('legal.clear_terms_confirm') }}
        </button>
      </template>
    </AppModal>

    <AppModal :open="bumping" :title="t('legal.bump_title')" @close="legal.cancelBump()">
      <div role="alert" class="alert alert-warning alert-soft">
        <Icon name="lucide:triangle-alert" size="18" class="text-warning" />
        <span class="text-sm text-base-content">{{ t('legal.bump_warning', { project: listed?.name ?? '' }) }}</span>
      </div>
      <p class="mt-3 text-sm font-medium">
        {{ t('legal.bump_versions', { from: version, to: nextVersion }) }}
      </p>
      <ErrorText :error="error" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="legal.cancelBump()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-warning btn-sm" :disabled="busy" @click="legal.confirmBump()">
          {{ t('legal.bump_confirm', { version: nextVersion }) }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
