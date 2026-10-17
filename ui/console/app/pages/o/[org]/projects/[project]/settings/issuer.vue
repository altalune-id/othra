<script setup lang="ts">
import IssuerHandleField from '~/components/project/issuer/IssuerHandleField.vue';
import { useIssuerHandle } from '~/components/project/issuer/useIssuerHandle';
import AppModal from '~/components/ui/AppModal.vue';
import CopyButton from '~/components/ui/CopyButton.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
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
const prefix = computed(() => {
  const url = issuerUrl.value;
  const h = project.value?.issuerHandle ?? '';
  return h && url.endsWith(h) ? url.slice(0, -h.length) : '';
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

    <div class="card border border-base-300 bg-base-100">
      <div class="card-body gap-4">
        <div class="space-y-1">
          <h2 class="text-xs font-semibold text-base-content/60">
            {{ t('issuer.url') }}
          </h2>
          <div class="flex items-center gap-2 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
            <code class="min-w-0 flex-1 font-mono text-sm break-all" data-testid="issuer-url">{{ issuerUrl }}</code>
            <CopyButton :value="issuerUrl" :label="t('common.copy')" class="shrink-0" />
          </div>
          <p class="text-xs text-base-content/60">
            {{ t('issuer.url_help') }}
          </p>
        </div>

        <form class="space-y-3" @submit.prevent="handle.ask()">
          <IssuerHandleField
            :model-value="value"
            :status="status"
            :prefix="prefix"
            :disabled="busy"
            :field-error="fieldError"
            :labels="fieldLabels"
            @update:model-value="handle.input"
          />
          <div v-if="locked" role="alert" class="alert alert-soft">
            <Icon name="lucide:lock" size="18" />
            <span class="text-sm">{{ t('issuer.locked_note') }}</span>
          </div>
          <template v-else>
            <div role="note" class="alert alert-info alert-soft">
              <Icon name="lucide:info" size="18" />
              <span class="text-sm">{{ t('issuer.unlocked_note') }}</span>
            </div>
            <ErrorText :error="error" />
            <div class="flex items-center gap-3">
              <button type="submit" class="btn btn-primary btn-sm" :disabled="!canSave">
                {{ t('common.save') }}
              </button>
              <span v-if="saved" class="text-sm text-success">{{ t('common.saved') }}</span>
            </div>
          </template>
        </form>
      </div>
    </div>

    <AppModal :open="confirming" :title="t('issuer.change_title')" @close="handle.cancel()">
      <p class="text-sm">
        {{ t('issuer.change_body', { old: issuerUrl, new: `${prefix}${value}` }) }}
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
