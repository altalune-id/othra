<script setup lang="ts">
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
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
  <div v-if="project" class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('issuer.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('issuer.subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="`${t('project.system_note')} ${t('project.system_issuer')}`" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />

    <div class="card border border-base-300 bg-base-100">
      <div class="card-body gap-4">
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
          <div v-if="locked" role="note" class="alert alert-soft">
            <Icon name="lucide:lock" size="18" class="text-info" />
            <span class="text-sm text-base-content">{{ t('issuer.locked_note') }}</span>
          </div>
          <template v-else>
            <div role="note" class="alert alert-soft">
              <Icon name="lucide:info" size="18" class="text-info" />
              <span class="text-sm text-base-content">{{ t('issuer.unlocked_note') }}</span>
            </div>
            <ErrorText :error="error" />
            <div class="flex items-center gap-3">
              <button type="submit" class="btn btn-primary btn-sm" :disabled="!canSave">
                {{ t('common.save') }}
              </button>
              <SavedNote v-if="saved" :label="t('common.saved')" />
            </div>
          </template>
        </form>
        <div v-else class="space-y-2" data-testid="issuer-readonly">
          <h2 class="fieldset-legend">
            {{ t('issuer.handle') }}
          </h2>
          <p class="flex items-center gap-2 font-mono text-sm">
            {{ project.issuerHandle }}
            <span v-if="locked" class="badge badge-sm badge-ghost gap-1">
              <Icon name="lucide:lock" size="12" />
              {{ t('issuer.locked') }}
            </span>
          </p>
          <p v-if="locked" role="note" class="text-sm text-base-content/60">
            {{ t('issuer.locked_note') }}
          </p>
        </div>
      </div>
    </div>

    <section v-if="discovery" class="card border border-base-300 bg-base-100" data-testid="discovery-card">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('issuer.discovery_title') }}
        </h2>
        <p class="text-sm text-base-content/70">
          {{ t('issuer.discovery_help') }}
        </p>
        <UrlField :label="t('issuer.discovery_oidc')" :value="discovery.oidc" open v-bind="urlLabels" />
        <UrlField :label="t('issuer.discovery_oauth')" :value="discovery.oauth" open v-bind="urlLabels" />
        <UrlField :label="t('keys.jwks_url')" :value="discovery.jwks" open v-bind="urlLabels" />
      </div>
    </section>

    <AppModal :open="confirming" :title="t('issuer.change_title')" @close="handle.cancel()">
      <p class="text-sm">
        {{ t('issuer.change_body', { old: issuerUrl, new: newUrl }) }}
      </p>
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="handle.cancel()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-warning btn-sm" :disabled="busy" @click="handle.save()">
          {{ t('issuer.change_confirm') }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
