<script setup lang="ts">
import type { ResourceServer } from '#gen/resourceserver/v1/resourceserver_pb';
import type { LinkRow } from '~/components/link/LinkList.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import FieldError from '#base/components/ui/FieldError.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import UrlField from '#base/components/ui/UrlField.vue';
import { useDates } from '#base/composables/useDates';
import { SecretGrace } from '#gen/client/v1/client_pb';
import ClientSecretDialog from '~/components/client/ClientSecretDialog.vue';
import ClientSecrets from '~/components/client/ClientSecrets.vue';
import RotateSecretDialog from '~/components/client/RotateSecretDialog.vue';
import { linkDialogLabels, linkListLabels } from '~/components/link/labels';
import LinkDialog from '~/components/link/LinkDialog.vue';
import LinkList from '~/components/link/LinkList.vue';
import { useLinks } from '~/components/link/useLinks';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import CimdCard from '~/components/resourceserver/CimdCard.vue';
import { cimdCardLabels, introspectionSecretLabels, scopeTableLabels, verificationCardLabels, verificationText } from '~/components/resourceserver/labels';
import PersonalTokenCard from '~/components/resourceserver/PersonalTokenCard.vue';
import { isHttpIdentifier } from '~/components/resourceserver/resourceUri';
import ScopeTable from '~/components/resourceserver/ScopeTable.vue';
import { admissible, isCimdLink, useCimd } from '~/components/resourceserver/useCimd';
import { introspectionEndpoint, usePersonalTokenAccess } from '~/components/resourceserver/usePersonalTokenAccess';
import { useResourceServer } from '~/components/resourceserver/useResourceServers';
import { metadataSnippet, verificationName, verificationStates } from '~/components/resourceserver/verification';
import VerificationCard from '~/components/resourceserver/VerificationCard.vue';
import { graceOptions, secretRows } from '~/components/secret/useSecrets';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const { date, moment } = useDates();
const orgs = useOrgsStore();
const projects = useProjectsStore();
const detail = useProjectDetailStore();
const canEdit = useCanEditProject();
await useAuthStore().load();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}
const projectId = listed.value.id;
const rsId = route.params.rs as string;
const base = `/o/${route.params.org}/projects/${route.params.project}/resource-servers`;

const page = useResourceServer(projectId, rsId);
await Promise.all([page.load(), detail.ensureLoaded(projectId)]);
if (page.notFound.value || !page.server.value) {
  throw createError({ statusCode: 404, statusMessage: t('apperror.RSV001'), fatal: true });
}
const server = computed(() => page.server.value as ResourceServer);
const { name, nameError, saving, saved, saveError, verifying, verified, verifyError, trustAsking, trustBusy, trustError, deleting, deleteBusy, deleteError } = page;
const scopeRows = page.scopes.rows;
const scopeDescriptions = page.scopes.descriptions;
const scopeErrors = page.scopes.errors;
const issuerUrl = computed(() => detail.get(projectId)?.issuerUrl ?? '');
const urlLabels = computed(() => ({ copyLabel: t('common.copy_label', { label: '{label}' }), openLabel: t('common.open_new_tab', { label: '{label}' }) }));
const status = computed(() => verificationName(server.value.state, server.value.trusted));
const text = computed(() => verificationText(server.value, status.value, issuerUrl.value, t, moment, verified.value));
const snippet = computed(() => (status.value === 'verified' || !issuerUrl.value ? '' : metadataSnippet(server.value.identifier, issuerUrl.value)));
const cardLabels = computed(() => verificationCardLabels(t));
const scopeLabels = computed(() => scopeTableLabels(t));
const ids = { name: useId(), nameErr: useId() };

const links = useLinks(projectId, { side: 'resourceServer', id: rsId, scopes: server.value.scopes });
const { editing, targetId, picked, pickError, unlinking, busy: linkBusy, error: linkError, available, target, scopeOptions } = links;
const linkRows = computed<LinkRow[]>(() => links.links.value.filter(l => !isCimdLink(l)).map(l => ({ id: l.clientId, name: l.clientName, detail: l.clientId, scopes: [...l.scopes] })));
const cimd = useCimd(projectId, rsId, next => page.replace(next), id => links.drop(id));
cimd.reset(server.value);
const { accept: cimdAccept, hosts: cimdHosts, allScopes: cimdAllScopes, picked: cimdPicked, revoking: cimdRevoking, revokeBusy: cimdRevokeBusy } = cimd;
const cimdText = computed(() => cimdCardLabels(t));
const cimdApps = computed(() => {
  return links.links.value.filter(isCimdLink).map(l => ({
    id: l.clientId,
    name: l.clientName,
    url: l.clientMetadataUrl,
    firstSeen: date(l.createdAt),
    fetched: moment(l.metadataFetchedAt),
  }));
});

function revokeApp(app: { id: string }) {
  const link = links.links.value.find(l => l.clientId === app.id);
  if (link) {
    cimd.askRevoke(link);
  }
}
const pat = usePersonalTokenAccess(projectId, rsId, () => server.value, next => page.replace(next));
const patSecrets = pat.secrets;
const { rotating: patRotating, grace: patGrace, revoking: patRevoking, busy: patSecretBusy, error: patSecretError, revealed: patRevealed } = patSecrets;
const patRows = computed(() => secretRows(server.value.introspectionSecrets).map(r => ({
  id: r.id,
  hint: r.hint,
  old: r.old,
  created: moment(r.created),
  expires: moment(r.expires),
})));
const patOldHint = computed(() => patRows.value.find(r => r.old)?.hint ?? '');
const graceText = computed(() => graceOptions.map(o => ({ value: o.value, label: t(o.labelKey) })));
const patText = computed(() => introspectionSecretLabels(t));

const linkText = computed(() => linkListLabels(t, 'resourceServer'));
const linkDialogText = computed(() => linkDialogLabels(t, 'resourceServer', editing.value === 'edit'));
const targetOptions = computed(() => available.value.map(x => ({ value: x.id, label: x.name })));
const unlinkingName = computed(() => unlinking.value?.clientName ?? '');
watch(() => server.value.scopes, (scopes) => {
  links.setOwnerScopes(scopes);
  cimd.catalog.value = [...scopes];
});

function editLink(row: LinkRow) {
  const link = links.links.value.find(l => l.clientId === row.id);
  if (link) {
    links.askEdit(link);
  }
}

function unlinkRow(row: LinkRow) {
  const link = links.links.value.find(l => l.clientId === row.id);
  if (link) {
    links.askUnlink(link);
  }
}

const verifyCard = ref<InstanceType<typeof VerificationCard>>();

async function verifyNow() {
  await page.verify();
  await nextTick();
  verifyCard.value?.focusVerify();
}

async function remove() {
  if (await page.confirmDelete()) {
    if (detail.get(projectId)) {
      void detail.load(projectId);
    }
    await navigateTo(base);
  }
}

onMounted(async () => {
  void links.load();
  if (route.query.verify !== '1') {
    return;
  }
  await navigateTo({ query: {} }, { replace: true });
  if (canEdit.value && !server.value.trusted) {
    await page.verify();
  }
});

useHead({ title: () => `${server.value.name} · ${t('rs.title')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <NuxtLink :to="base" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
          <Icon name="lucide:chevron-left" size="14" />
          {{ t('rs.title') }}
        </NuxtLink>
        <h1 class="mt-1 text-2xl font-bold break-words">
          {{ server.name }}
        </h1>
      </div>
      <button v-if="canEdit" type="button" class="btn btn-ghost btn-sm max-sm:btn-block" @click="page.askDelete()">
        <Icon name="lucide:trash-2" size="16" class="text-error" />
        {{ t('common.delete') }}
      </button>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" />

    <section class="card border border-base-300 bg-base-100" data-testid="rs-details">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('rs.details') }}
        </h2>
        <UrlField :label="t('rs.uri')" :value="server.identifier" size="sm" testid="rs-identifier" v-bind="urlLabels" />
        <form v-if="canEdit" novalidate class="fieldset" @submit.prevent="page.save('name')">
          <label :for="ids.name" class="fieldset-legend">{{ t('common.name') }}</label>
          <div class="flex flex-wrap items-center gap-2">
            <input
              :id="ids.name"
              v-model="name"
              type="text"
              class="input min-w-0 flex-1"
              :class="{ 'input-error': nameError }"
              maxlength="100"
              :disabled="saving === 'name'"
              :aria-invalid="!!nameError"
              :aria-describedby="ids.nameErr"
            >
            <button type="submit" class="btn btn-sm" :disabled="saving === 'name'">
              <span v-if="saving === 'name'" class="loading loading-spinner loading-xs" />
              {{ t('common.save') }}
            </button>
          </div>
          <FieldError :id="ids.nameErr" :message="nameError" />
          <ErrorText :error="saveError?.part === 'name' ? saveError.error : null" />
          <SavedNote v-if="saved === 'name'" :label="t('common.saved')" />
        </form>
        <dl v-else class="grid grid-cols-[auto_1fr] gap-x-4 text-sm">
          <dt class="text-base-content/60">
            {{ t('common.name') }}
          </dt>
          <dd>{{ server.name }}</dd>
        </dl>
      </div>
    </section>

    <VerificationCard
      ref="verifyCard"
      :status="status"
      :status-label="t(verificationStates[status].labelKey)"
      :body="text.body"
      :metadata-url="server.metadataUrl"
      :last-check="text.lastCheck"
      :verified-at="text.verifiedAt"
      :failure="text.failure"
      :recheck="text.recheck"
      :snippet="snippet"
      :can-edit="canEdit"
      :trust-available="server.trustAvailable"
      :http="isHttpIdentifier(server.identifier)"
      :verifying="verifying"
      :verify-error="verifyError"
      :passed="text.passed"
      :trust-busy="trustBusy"
      :trust-error="trustAsking ? null : trustError"
      :labels="cardLabels"
      @verify="verifyNow"
      @toggle-trust="page.toggleTrust()"
    />

    <section class="card border border-base-300 bg-base-100" data-testid="rs-scopes">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('rs.scopes') }}
        </h2>
        <form v-if="canEdit" novalidate class="space-y-2" @submit.prevent="page.save('scopes')">
          <ScopeTable
            v-model:rows="scopeRows"
            v-model:descriptions="scopeDescriptions"
            :errors="scopeErrors"
            :disabled="saving === 'scopes'"
            :full="page.scopes.full.value"
            :labels="scopeLabels"
            @add="page.scopes.add()"
            @remove="page.scopes.remove"
            @edit="page.scopes.clearRow"
          />
          <ErrorText :error="saveError?.part === 'scopes' ? saveError.error : null" />
          <div class="flex items-center gap-3">
            <button type="submit" class="btn btn-primary btn-sm" :disabled="saving === 'scopes'">
              <span v-if="saving === 'scopes'" class="loading loading-spinner loading-xs" />
              {{ t('rs.save_scopes') }}
            </button>
            <SavedNote v-if="saved === 'scopes'" :label="t('common.saved')" />
          </div>
        </form>
        <ScopeTable v-else :rows="server.scopes" :descriptions="server.scopes.map(s => server.scopeDescriptions[s] ?? '')" :editable="false" />
      </div>
    </section>

    <CimdCard
      v-model:accept="cimdAccept"
      v-model:hosts="cimdHosts"
      v-model:all-scopes="cimdAllScopes"
      v-model:picked="cimdPicked"
      :can-edit="canEdit"
      :locked="!admissible(server)"
      :catalog="cimd.catalog.value"
      :apps="cimdApps"
      :busy="cimd.busy.value"
      :saved="cimd.saved.value"
      :error="cimd.error.value"
      :hosts-error="cimd.hostsError.value"
      :labels="cimdText"
      @save="cimd.save()"
      @revoke="revokeApp"
    />

    <PersonalTokenCard
      :accept="server.acceptPersonalTokens"
      :can-edit="canEdit"
      :lock-reason="pat.lockReason.value"
      :busy="pat.busy.value"
      :error="pat.error.value"
      :endpoint="introspectionEndpoint(issuerUrl)"
      :copy-label="urlLabels.copyLabel"
      @set="pat.setAccept"
    />

    <ClientSecrets
      :rows="patRows"
      :can-edit="canEdit"
      :busy="patSecretBusy && !patRotating && !patRevoking"
      :error="patRotating || patRevoking ? null : patSecretError"
      :labels="patText.card"
      @generate="patSecrets.generate()"
      @rotate="patSecrets.askRotate()"
      @revoke="patSecrets.askRevoke"
    />

    <RotateSecretDialog
      v-model:grace="patGrace"
      :open="patRotating"
      :options="graceText"
      :now-value="SecretGrace.NOW"
      :old-hint="patOldHint"
      :busy="patSecretBusy"
      :error="patSecretError"
      :labels="patText.rotate"
      @confirm="patSecrets.confirmRotate()"
      @close="patSecrets.cancelRotate()"
    />

    <AppModal :open="!!patRevoking" :title="patText.revokeTitle" @close="patSecrets.cancelRevoke()">
      <p class="text-sm">
        {{ patText.revokeBody.replace('{hint}', patRevoking?.hint ?? '') }}
      </p>
      <ErrorText :error="patSecretError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="patSecretBusy" @click="patSecrets.cancelRevoke()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="patSecretBusy" @click="patSecrets.confirmRevoke()">
          {{ t('secrets.revoke_old') }}
        </button>
      </template>
    </AppModal>

    <ClientSecretDialog
      :open="!!patRevealed"
      :owner-id="patRevealed?.ownerId ?? server.id"
      :secret="patRevealed?.secret ?? ''"
      :labels="patText.dialog"
      @done="patSecrets.forget()"
    />

    <LinkList
      :rows="linkRows"
      :can-edit="canEdit"
      :loading="links.loading.value"
      :load-error="links.loadError.value"
      :labels="linkText"
      @link="links.askLink()"
      @edit="editLink"
      @unlink="unlinkRow"
    />

    <AppModal :open="trustAsking" :title="t('rs.trust_title', { uri: server.identifier })" @close="page.cancelTrust()">
      <p class="flex items-start gap-1.5 text-sm text-base-content">
        <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" />
        {{ t('rs.trust_body') }}
      </p>
      <ErrorText :error="trustError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="trustBusy" autofocus @click="page.cancelTrust()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-warning btn-sm" :disabled="trustBusy" @click="page.setTrust(true)">
          <span v-if="trustBusy" class="loading loading-spinner loading-xs" />
          {{ t('rs.trust') }}
        </button>
      </template>
    </AppModal>

    <LinkDialog
      v-model:target-id="targetId"
      v-model:picked="picked"
      :open="!!editing"
      :mode="editing === 'edit' ? 'edit' : 'new'"
      :targets="targetOptions"
      :fixed-name="target?.name ?? ''"
      :scope-options="scopeOptions"
      :pick-error="pickError"
      :busy="linkBusy"
      :error="linkError"
      :labels="linkDialogText"
      @target="links.pickTarget"
      @confirm="links.confirmEdit()"
      @close="links.cancelEdit()"
    />

    <AppModal :open="!!unlinking" :title="t('links.unlink_title', { name: unlinkingName })" @close="links.cancelUnlink()">
      <p class="text-sm">
        {{ t('links.unlink_body', { client: unlinkingName, rs: server.name }) }}
      </p>
      <ErrorText :error="linkError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="linkBusy" autofocus @click="links.cancelUnlink()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="linkBusy" @click="links.confirmUnlink()">
          <span v-if="linkBusy" class="loading loading-spinner loading-xs" />
          {{ t('links.unlink') }}
        </button>
      </template>
    </AppModal>

    <AppModal :open="!!cimdRevoking" :title="t('rs.cimd_revoke_title', { name: cimdRevoking?.clientName ?? '' })" @close="cimd.cancelRevoke()">
      <p class="text-sm">
        {{ t('rs.cimd_revoke_body', { name: cimdRevoking?.clientName ?? '' }) }}
      </p>
      <ErrorText :error="cimd.revokeError.value" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="cimdRevokeBusy" autofocus @click="cimd.cancelRevoke()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="cimdRevokeBusy" @click="cimd.confirmRevoke()">
          <span v-if="cimdRevokeBusy" class="loading loading-spinner loading-xs" />
          {{ t('rs.cimd_revoke') }}
        </button>
      </template>
    </AppModal>

    <AppModal :open="deleting" :title="t('rs.delete_title', { name: server.name })" @close="page.cancelDelete()">
      <p class="text-sm">
        {{ t('rs.delete_body', { uri: server.identifier }) }}
      </p>
      <ErrorText :error="deleteError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="deleteBusy" autofocus @click="page.cancelDelete()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="deleteBusy" @click="remove">
          <span v-if="deleteBusy" class="loading loading-spinner loading-xs" />
          {{ t('common.delete') }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
