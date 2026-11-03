<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import Section from '#base/components/ui/Section.vue';
import UrlField from '#base/components/ui/UrlField.vue';
import { discoveryUrls } from '~/components/project/issuer/discovery';
import IssuerHandleField from '~/components/project/issuer/IssuerHandleField.vue';
import { useIssuerHandle } from '~/components/project/issuer/useIssuerHandle';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const detail = useProjectDetailStore();
const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
const guard = useProjectGuard(() => listed.value);
const canEdit = computed(() => guard.value.canConfigure);
await useAuthStore().load();
if (!org.value || !listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}
await detail.ensureLoaded(listed.value.id);
const project = computed(() => detail.get(listed.value?.id ?? ''));
if (!project.value) {
  throw createError({ statusCode: 502, statusMessage: t('error.load_project'), fatal: true });
}

const handle = useIssuerHandle({
  orgId: org.value.id,
  projectId: project.value.id,
  initial: project.value.issuerHandle,
  locked: project.value.issuerHandleLocked,
});
const { value, status, canSave, confirming, busy, saved, error, fieldError } = handle;

const issuerUrl = computed(() => project.value?.issuerUrl ?? '');
const discovery = computed(() => discoveryUrls(issuerUrl.value));
const urlLabels = computed(() => ({ copyLabel: t('common.copy_label', { label: '{label}' }), openLabel: t('common.open_new_tab', { label: '{label}' }) }));
const newUrl = computed(() => {
  const h = project.value?.issuerHandle ?? '';
  return h && issuerUrl.value.endsWith(h) ? `${issuerUrl.value.slice(0, -h.length)}${value.value}` : value.value;
});
const locked = computed(() => status.value === 'locked');
const fieldLabels = computed(() => ({
  handle: t('issuer.handle'),
  help: t('issuer.handle_help'),
  checking: t('issuer.checking'),
  available: t('issuer.available'),
  taken: t('issuer.taken'),
  reserved: t('issuer.reserved'),
  invalid: t('issuer.invalid'),
  invalidFormat: t('apperror.PRJ006_format'),
  invalidLength: t('apperror.PRJ006_length', { min: 3, max: 40 }),
  locked: t('issuer.locked'),
  error: t('issuer.check_failed'),
}));

useHead({ title: () => `${project.value?.name ?? t('project.title')} · ${t('issuer.title')} · Othra` });
</script>

<template>
  <div v-if="project" class="flex max-w-2xl flex-col gap-5">
    <PageHeader :title="t('issuer.title')" :description="t('issuer.subtitle')" />
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="`${t('project.system_note')} ${t('project.system_issuer')}`" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <div class="flex flex-col gap-4 rounded-box border border-base-300 bg-base-100 p-4">
      <UrlField :label="t('issuer.url')" :value="issuerUrl" :help="t('issuer.url_help')" testid="issuer-url" v-bind="urlLabels" />

      <form v-if="canEdit" novalidate class="space-y-3" @submit.prevent="handle.ask()">
        <IssuerHandleField
          :model-value="value"
          :status="status"
          prefix="/p/"
          :disabled="busy"
          :field-error="fieldError"
          :labels="fieldLabels"
          @update:model-value="handle.input"
        />
        <div v-if="locked" role="note" class="alert items-start text-ui">
          <Icon name="lucide:lock" size="16" class="shrink-0 text-info" aria-hidden="true" />
          <span>{{ t('issuer.locked_note') }}</span>
        </div>
        <template v-else>
          <div role="note" class="alert items-start text-ui">
            <Icon name="lucide:info" size="16" class="shrink-0 text-info" aria-hidden="true" />
            <span>{{ t('issuer.unlocked_note') }}</span>
          </div>
          <ErrorText :error="error" />
          <div class="flex items-center gap-3">
            <AppButton type="submit" size="sm" :disabled="!canSave">
              {{ t('common.save') }}
            </AppButton>
            <SavedNote v-if="saved" :label="t('common.saved')" />
          </div>
        </template>
      </form>
      <div v-else class="space-y-2" data-testid="issuer-readonly">
        <h2 class="label text-ui font-semibold text-base-content">
          {{ t('issuer.handle') }}
        </h2>
        <p class="flex items-center gap-2 font-mono text-xs">
          {{ project.issuerHandle }}
          <span v-if="locked" class="badge badge-sm badge-ghost gap-1">
            <Icon name="lucide:lock" size="12" aria-hidden="true" />
            {{ t('issuer.locked') }}
          </span>
        </p>
        <p v-if="locked" role="note" class="text-ui text-base-content/70">
          {{ t('issuer.locked_note') }}
        </p>
      </div>
    </div>

    <Section v-if="discovery" :title="t('issuer.discovery_title')" :description="t('issuer.discovery_help')" data-testid="discovery-card">
      <UrlField :label="t('issuer.discovery_oidc')" :value="discovery.oidc" open v-bind="urlLabels" />
      <UrlField :label="t('issuer.discovery_oauth')" :value="discovery.oauth" open v-bind="urlLabels" />
      <UrlField :label="t('keys.jwks_url')" :value="discovery.jwks" open v-bind="urlLabels" />
    </Section>

    <AppModal :open="confirming" :title="t('issuer.change_title')" @close="handle.cancel()">
      <p class="text-ui break-words">
        {{ t('issuer.change_body', { old: issuerUrl, new: newUrl }) }}
      </p>
      <template #footer>
        <AppButton variant="ghost" size="sm" :disabled="busy" autofocus @click="handle.cancel()">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton variant="warning" size="sm" :busy="busy" @click="handle.save()">
          {{ t('issuer.change_confirm') }}
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>
