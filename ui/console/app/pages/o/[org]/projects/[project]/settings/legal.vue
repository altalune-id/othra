<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import Section from '#base/components/ui/Section.vue';
import { useDates } from '#base/composables/useDates';
import { useLegal } from '~/components/project/legal/useLegal';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
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
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const legal = useLegal(listed.value.id);
const { termsUrl, privacyUrl, hasTerms, version, nextVersion, publishedAt, urlChangedHint, busy, saved, clearing, bumping, bumpedTo, error, fieldErrors } = legal;
const ids = { terms: useId(), termsHint: useId(), termsErr: useId(), privacy: useId(), privacyHelp: useId(), privacyErr: useId() };
const published = computed(() => date(publishedAt.value));

const canEdit = useCanEditProject();
await Promise.all([legal.load(), useAuthStore().load()]);

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('legal.title')} · Othra` });
</script>

<template>
  <div class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('legal.title')" :description="t('legal.subtitle')" />
    <ProjectReadOnlyNote v-if="!canEdit" />

    <form v-if="canEdit" novalidate class="flex flex-col gap-4 rounded-box border border-base-300 bg-base-100 p-4" @submit.prevent="legal.save()">
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('legal.terms_url') }}</span>
          <input :id="ids.terms" v-model="termsUrl" type="url" class="input input-sm w-full text-ui" placeholder="https://example.com/terms" :aria-describedby="urlChangedHint ? `${ids.termsHint} ${ids.termsErr}` : ids.termsErr" :disabled="busy">
        </label>
        <FieldError :id="ids.termsErr" :message="fieldErrors.terms_url" />
        <p v-if="urlChangedHint" :id="ids.termsHint" class="flex items-start gap-1 text-xs text-base-content/70">
          <Icon name="lucide:info" size="14" class="shrink-0 text-warning" aria-hidden="true" />
          {{ t('legal.url_changed_hint') }}
        </p>
      </div>
      <div class="fieldset">
        <label class="flex flex-col gap-1">
          <span class="label text-ui text-base-content">{{ t('legal.privacy_url') }}</span>
          <input :id="ids.privacy" v-model="privacyUrl" type="url" class="input input-sm w-full text-ui" placeholder="https://example.com/privacy" :aria-describedby="`${ids.privacyHelp} ${ids.privacyErr}`" :disabled="busy">
        </label>
        <FieldError :id="ids.privacyErr" :message="fieldErrors.privacy_url" />
        <p :id="ids.privacyHelp" class="label whitespace-normal text-xs text-base-content/70">
          {{ t('legal.privacy_help') }}
        </p>
      </div>
      <ErrorText v-if="!bumping" :error="error" />
      <div class="flex items-center gap-3">
        <AppButton type="submit" size="sm" :disabled="busy">
          {{ busy ? t('common.saving') : t('common.save') }}
        </AppButton>
        <SavedNote v-if="saved" :label="t('common.saved')" />
      </div>
    </form>
    <dl v-else class="flex flex-col gap-4 rounded-box border border-base-300 bg-base-100 p-4 text-ui" data-testid="legal-readonly">
      <div>
        <dt class="label text-ui text-base-content">
          {{ t('legal.terms_url') }}
        </dt>
        <dd class="break-all">
          <template v-if="termsUrl">
            {{ termsUrl }}
          </template>
          <span v-else class="text-base-content/70">—</span>
        </dd>
      </div>
      <div>
        <dt class="label text-ui text-base-content">
          {{ t('legal.privacy_url') }}
        </dt>
        <dd class="break-all">
          <template v-if="privacyUrl">
            {{ privacyUrl }}
          </template>
          <span v-else class="text-base-content/70">—</span>
        </dd>
      </div>
    </dl>

    <Section :title="t('legal.acceptance_title')">
      <template v-if="hasTerms">
        <p class="text-ui font-medium" data-testid="terms-version">
          {{ t('legal.current_version', { version, date: published }) }}
        </p>
        <p class="text-ui text-base-content/70">
          {{ t('legal.acceptance_body') }}
        </p>
        <SavedNote v-if="bumpedTo" :label="t('legal.bump_done', { version: bumpedTo })" />
        <div v-if="canEdit" class="flex justify-end">
          <AppButton variant="outline" size="sm" :disabled="busy" @click="legal.askBump()">
            <Icon name="lucide:triangle-alert" size="16" class="shrink-0 text-warning" aria-hidden="true" />
            {{ t('legal.bump') }}
          </AppButton>
        </div>
      </template>
      <p v-else class="text-ui text-base-content/70">
        {{ canEdit ? t('legal.no_terms') : t('legal.no_terms_readonly') }}
      </p>
    </Section>

    <AppModal :open="clearing" :title="t('legal.clear_terms_title')" @close="legal.cancelClear()">
      <p class="text-ui">
        {{ t('legal.clear_terms_body') }}
      </p>
      <template #footer>
        <AppButton variant="ghost" size="sm" autofocus @click="legal.cancelClear()">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton variant="warning" size="sm" :busy="busy" @click="legal.confirmClear()">
          {{ t('legal.clear_terms_confirm') }}
        </AppButton>
      </template>
    </AppModal>

    <AppModal :open="bumping" :title="t('legal.bump_title')" @close="legal.cancelBump()">
      <div role="alert" class="alert alert-warning alert-soft items-start text-ui">
        <Icon name="lucide:triangle-alert" size="16" class="shrink-0 text-warning" aria-hidden="true" />
        <span>{{ t('legal.bump_warning', { project: listed?.name ?? '' }) }}</span>
      </div>
      <p class="text-ui font-medium">
        {{ t('legal.bump_versions', { from: version, to: nextVersion }) }}
      </p>
      <ErrorText :error="error" />
      <template #footer>
        <AppButton variant="ghost" size="sm" :disabled="busy" autofocus @click="legal.cancelBump()">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton variant="warning" size="sm" :busy="busy" @click="legal.confirmBump()">
          {{ t('legal.bump_confirm', { version: nextVersion }) }}
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>
