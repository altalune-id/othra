<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import LoadError from '#base/components/ui/LoadError.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import Section from '#base/components/ui/Section.vue';
import { keyRows } from '~/components/project/keys/keyRows';
import SigningKeyList from '~/components/project/keys/SigningKeyList.vue';
import { formatWhen, shortKid, useSigningKeys } from '~/components/project/keys/useSigningKeys';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
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
  caption: t('keys.title'),
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
  <div class="flex max-w-4xl flex-col gap-5">
    <PageHeader :title="t('keys.title')" :description="t('keys.subtitle')" />
    <ProjectReadOnlyNote v-if="!canEdit" :text="t('keys.members_note')" />
    <div v-if="noActive" role="note" class="alert items-start text-ui">
      <Icon name="lucide:triangle-alert" size="16" class="shrink-0 text-warning" aria-hidden="true" />
      <span>{{ canEdit ? t('keys.no_active') : t('keys.no_active_member') }}</span>
    </div>

    <LoadError v-if="loadError" :error="loadError" :busy="loading" :retry-label="t('common.retry')" @retry="keys.load()" />
    <SigningKeyList v-else :rows="rows" :loading="loading" :labels="listLabels" data-testid="keys-card" />

    <Section v-if="canEdit && loaded" :title="t('keys.rotation_title')" data-testid="rotation-card">
      <p v-if="waitingUntil && !noActive" role="status" class="flex items-center gap-2 text-ui">
        <Icon name="lucide:clock" size="16" class="shrink-0 text-info" aria-hidden="true" />
        {{ waitingText }}
      </p>
      <AppButton v-else size="sm" class="self-start max-sm:btn-block max-sm:self-stretch" @click="keys.askRotate()">
        <Icon name="lucide:refresh-cw" size="16" class="shrink-0" aria-hidden="true" />
        {{ t('keys.rotate') }}
      </AppButton>
      <SavedNote v-if="rotatedText" :label="rotatedText" />
      <div class="divider my-0" />
      <p class="text-ui text-base-content/70">
        {{ t('keys.emergency') }}
      </p>
      <div class="flex items-center gap-1 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
        <code class="min-w-0 flex-1 font-mono text-xs break-all">{{ command }}</code>
        <CopyButton :value="command" :label="t('common.copy')" class="shrink-0" />
      </div>
    </Section>

    <AppModal :open="confirming" :title="t('keys.rotate_title')" @close="keys.cancelRotate()">
      <p class="text-ui">
        {{ t('keys.rotate_body') }}
      </p>
      <ErrorText :error="error" />
      <template #footer>
        <AppButton variant="ghost" size="sm" :disabled="busy" autofocus @click="keys.cancelRotate()">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton size="sm" :busy="busy" @click="keys.confirmRotate()">
          {{ t('keys.rotate_confirm') }}
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>
