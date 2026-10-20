<script setup lang="ts">
import { keyRows } from '~/components/project/keys/keyRows';
import SigningKeyList from '~/components/project/keys/SigningKeyList.vue';
import { formatWhen, shortKid, useSigningKeys } from '~/components/project/keys/useSigningKeys';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import AppModal from '~/components/ui/AppModal.vue';
import CopyButton from '~/components/ui/CopyButton.vue';
import ErrorText from '~/components/ui/ErrorText.vue';
import SavedNote from '~/components/ui/SavedNote.vue';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t, locale } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const canEdit = useCanEditProject();
await useAuthStore().load();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const keys = useSigningKeys(listed.value.id);
const { loading, loaded, loadError, confirming, busy, error, rotated, waitingUntil, noActive } = keys;
const rows = computed(() => keyRows(keys.keys.value, { t, locale: locale.value }));
const listLabels = computed(() => ({
  kid: t('keys.col_kid'),
  alg: t('keys.col_alg'),
  status: t('keys.col_status'),
  created: t('keys.col_created'),
  retires: t('keys.col_retires'),
  next: t('keys.status_next'),
  active: t('keys.status_active'),
  retiring: t('keys.status_retiring'),
  retired: t('keys.status_retired'),
  unknown: '—',
  copy: t('common.copy'),
  retiredNote: t('keys.retired_note'),
}));
const waitingText = computed(() => (waitingUntil.value ? t('keys.rotate_waiting', { time: formatWhen(waitingUntil.value, locale.value) }) : ''));
const rotatedText = computed(() => {
  const r = rotated.value;
  if (!r) {
    return '';
  }
  return r.activatesAt
    ? t('keys.rotated', { kid: shortKid(r.kid), time: formatWhen(r.activatesAt, locale.value) })
    : t('keys.rotated_active', { kid: shortKid(r.kid) });
});
const command = computed(() => `othra project keys rotate ${listed.value?.id} --now`);

onMounted(() => keys.load());
onBeforeUnmount(() => keys.dispose());

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('keys.title')} · Othra` });
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('keys.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('keys.subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" :text="t('keys.members_note')" />
    <div v-if="noActive" role="note" class="alert alert-soft">
      <Icon name="lucide:triangle-alert" size="18" class="text-warning" />
      <span class="text-sm text-base-content">{{ canEdit ? t('keys.no_active') : t('keys.no_active_member') }}</span>
    </div>

    <section class="card border border-base-300 bg-base-100" data-testid="keys-card">
      <div class="card-body gap-2">
        <template v-if="loadError">
          <ErrorText :error="loadError" />
          <div>
            <button type="button" class="btn btn-sm" @click="keys.load()">
              {{ t('common.retry') }}
            </button>
          </div>
        </template>
        <SigningKeyList v-else :rows="rows" :loading="loading" :labels="listLabels" />
      </div>
    </section>

    <section v-if="canEdit && loaded" class="card border border-base-300 bg-base-100" data-testid="rotation-card">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('keys.rotation_title') }}
        </h2>
        <p v-if="waitingUntil && !noActive" role="status" class="flex items-center gap-2 text-sm text-base-content">
          <Icon name="lucide:clock" size="16" class="shrink-0 text-info" />
          {{ waitingText }}
        </p>
        <button v-else type="button" class="btn btn-primary btn-sm self-start max-sm:btn-block max-sm:self-stretch" @click="keys.askRotate()">
          <Icon name="lucide:refresh-cw" size="16" />
          {{ t('keys.rotate') }}
        </button>
        <SavedNote v-if="rotatedText" :label="rotatedText" />
        <div class="divider my-1" />
        <p class="text-sm text-base-content/70">
          {{ t('keys.emergency') }}
        </p>
        <div class="flex items-center gap-1 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
          <code class="min-w-0 flex-1 font-mono text-sm break-all">{{ command }}</code>
          <CopyButton :value="command" :label="t('common.copy')" class="shrink-0" />
        </div>
      </div>
    </section>

    <AppModal :open="confirming" :title="t('keys.rotate_title')" @close="keys.cancelRotate()">
      <p class="text-sm">
        {{ t('keys.rotate_body') }}
      </p>
      <ErrorText :error="error" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="keys.cancelRotate()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary btn-sm" :disabled="busy" @click="keys.confirmRotate()">
          <span v-if="busy" class="loading loading-spinner loading-xs" />
          {{ t('keys.rotate_confirm') }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
