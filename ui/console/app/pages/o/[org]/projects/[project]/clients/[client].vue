<script setup lang="ts">
import type { Client } from '#gen/client/v1/client_pb';
import type { LinkRow } from '~/components/link/LinkList.vue';
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import SavedNote from '#base/components/ui/SavedNote.vue';
import UrlField from '#base/components/ui/UrlField.vue';
import { useDates } from '#base/composables/useDates';
import { ConsentPolicy, SecretGrace } from '#gen/client/v1/client_pb';
import ClientForm from '~/components/client/ClientForm.vue';
import ClientSecretDialog from '~/components/client/ClientSecretDialog.vue';
import ClientSecrets from '~/components/client/ClientSecrets.vue';
import { clientKinds, isCimdClient, kindName } from '~/components/client/kinds';
import { clientFormLabels, kindOptions, rotateLabels, secretDialogLabels, secretsLabels } from '~/components/client/labels';
import RotateSecretDialog from '~/components/client/RotateSecretDialog.vue';
import { useClientForm } from '~/components/client/useClientForm';
import { useClient } from '~/components/client/useClients';
import { useClientSecrets } from '~/components/client/useClientSecrets';
import { linkDialogLabels, linkListLabels } from '~/components/link/labels';
import LinkDialog from '~/components/link/LinkDialog.vue';
import LinkList from '~/components/link/LinkList.vue';
import { useLinks } from '~/components/link/useLinks';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import { useCanEditProject } from '~/components/project/useProjectAccess';
import { verificationName, verificationStates } from '~/components/resourceserver/verification';
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
const clientId = route.params.client as string;
const base = `/o/${route.params.org}/projects/${route.params.project}`;

const page = useClient(projectId, clientId);
await Promise.all([page.load(), detail.ensureLoaded(projectId)]);
if (page.notFound.value || !page.client.value) {
  throw createError({ statusCode: 404, statusMessage: t('apperror.CLT001'), fatal: true });
}
const client = computed(() => page.client.value as Client);
const kind = computed(() => clientKinds[kindName(client.value.kind)]);
const cimd = computed(() => isCimdClient(client.value));
const editable = computed(() => canEdit.value && !cimd.value);
const issuerUrl = computed(() => detail.get(projectId)?.issuerUrl ?? '');
const urlLabels = computed(() => ({ copyLabel: t('common.copy_label', { label: '{label}' }), openLabel: t('common.open_new_tab', { label: '{label}' }) }));
const fetched = computed(() => moment(client.value.metadataFetchedAt));
const created = computed(() => date(client.value.createdAt));
const consentLabel = computed(() => t(client.value.consentPolicy === ConsentPolicy.IMPLICIT ? 'clients.consent_implicit' : 'clients.consent_require'));

const form = useClientForm(projectId, client.value);
const { fields, warnings, uriErrors, nameError, busy, error, done } = form;
const formLabels = computed(() => clientFormLabels(t, 'edit'));
const kinds = computed(() => kindOptions(t));

async function save() {
  const next = await form.submit();
  if (next) {
    page.replace(next);
  }
}

const secretsCard = ref<InstanceType<typeof ClientSecrets>>();
const secrets = useClientSecrets(projectId, clientId, next => page.replace(next));
const { rotating, grace, revoking, busy: secretBusy, error: secretError, revealed } = secrets;
const rows = computed(() => secretRows(client.value.secrets).map(r => ({
  id: r.id,
  hint: r.hint,
  old: r.old,
  created: moment(r.created),
  expires: moment(r.expires),
})));
const oldHint = computed(() => rows.value.find(r => r.old)?.hint ?? '');
const options = computed(() => graceOptions.map(o => ({ value: o.value, label: t(o.labelKey) })));
const secretsText = computed(() => secretsLabels(t));
const rotateText = computed(() => rotateLabels(t));
const dialogText = computed(() => secretDialogLabels(t));

async function secretSaved() {
  secrets.forget();
  await nextTick();
  secretsCard.value?.focusTitle();
}

async function revoke() {
  await secrets.confirmRevoke();
  if (!revoking.value) {
    await nextTick();
    secretsCard.value?.focusAction();
  }
}

const links = useLinks(projectId, { side: 'client', id: clientId });
const { editing, targetId, picked, pickError, unlinking, busy: linkBusy, error: linkError, available, target, scopeOptions } = links;
const linkRows = computed<LinkRow[]>(() => links.links.value.map((l) => {
  const name = verificationName(l.resourceServerState, l.resourceServerTrusted);
  return { id: l.resourceServerId, name: l.resourceServerName, detail: l.resourceServerIdentifier, scopes: [...l.scopes], status: { name, label: t(verificationStates[name].labelKey) } };
}));
const linkText = computed(() => linkListLabels(t, 'client'));
const linkDialogText = computed(() => linkDialogLabels(t, 'client', editing.value === 'edit'));
const targetOptions = computed(() => available.value.map(x => ({ value: x.id, label: x.name })));
const unlinkingName = computed(() => unlinking.value?.resourceServerName ?? '');

function editLink(row: LinkRow) {
  const link = links.links.value.find(l => l.resourceServerId === row.id);
  if (link) {
    links.askEdit(link);
  }
}

function unlinkRow(row: LinkRow) {
  const link = links.links.value.find(l => l.resourceServerId === row.id);
  if (link) {
    links.askUnlink(link);
  }
}

async function remove() {
  if (await page.confirmDelete()) {
    if (detail.get(projectId)) {
      void detail.load(projectId);
    }
    await navigateTo(`${base}/clients`);
  }
}

function warnLeave(event: BeforeUnloadEvent) {
  if (revealed.value) {
    event.preventDefault();
  }
}

onBeforeRouteLeave(() => !revealed.value);
onMounted(async () => {
  window.addEventListener('beforeunload', warnLeave);
  void links.load();
  if (route.query.focus !== 'secrets') {
    return;
  }
  await navigateTo({ query: {} }, { replace: true });
  await nextTick();
  secretsCard.value?.focusTitle();
});
onBeforeUnmount(() => window.removeEventListener('beforeunload', warnLeave));

useHead({ title: () => `${client.value.name} · ${t('clients.title')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <NuxtLink :to="`${base}/clients`" class="inline-flex items-center gap-1 text-xs text-base-content/60 transition-colors hover:text-base-content">
          <Icon name="lucide:chevron-left" size="14" />
          {{ t('clients.title') }}
        </NuxtLink>
        <h1 class="mt-1 flex flex-wrap items-center gap-2 text-2xl font-bold break-words">
          {{ client.name }}
          <span v-if="cimd" class="badge badge-soft badge-warning badge-sm font-normal" data-testid="client-cimd-badge">{{ t('clients.cimd_badge') }}</span>
        </h1>
      </div>
      <button v-if="canEdit" type="button" class="btn btn-ghost btn-sm max-sm:btn-block" @click="page.askDelete()">
        <Icon :name="cimd ? 'lucide:ban' : 'lucide:trash-2'" size="16" class="text-error" />
        {{ cimd ? t('rs.cimd_revoke') : t('common.delete') }}
      </button>
    </div>
    <ProjectReadOnlyNote v-if="!canEdit" />

    <section class="card border border-base-300 bg-base-100" data-testid="client-details">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('clients.details') }}
        </h2>
        <UrlField
          v-if="cimd"
          :label="t('rs.metadata_url')"
          :value="client.metadataUrl"
          :help="fetched ? `${t('rs.cimd_fetched')} ${fetched}` : ''"
          size="sm"
          open
          testid="client-metadata-url"
          v-bind="urlLabels"
        />
        <UrlField v-else :label="t('clients.client_id')" :value="client.id" :help="t('clients.client_id_help')" size="sm" testid="client-id" v-bind="urlLabels" />
        <UrlField v-if="issuerUrl" :label="t('clients.issuer')" :value="issuerUrl" size="sm" v-bind="urlLabels" />
        <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt class="text-base-content/60">
            {{ t('clients.kind') }}
          </dt>
          <dd>{{ t(kind.labelKey) }}</dd>
          <template v-if="kind.secret">
            <dt class="text-base-content/60">
              {{ t('clients.auth_method') }}
            </dt>
            <dd><code class="font-mono text-xs" data-testid="client-auth-method">{{ client.tokenEndpointAuthMethod }}</code></dd>
            <dt class="text-base-content/60">
              {{ t('clients.pkce') }}
            </dt>
            <dd v-if="client.pkceRequired">
              {{ t('clients.pkce_on') }}
            </dd>
            <dd v-else class="flex items-start gap-1" data-testid="client-pkce-off">
              <Icon name="lucide:triangle-alert" size="14" class="mt-0.5 shrink-0 text-warning" />
              {{ t('clients.pkce_off') }}
            </dd>
          </template>
          <dt class="text-base-content/60">
            {{ t('clients.scopes') }}
          </dt>
          <dd class="flex flex-wrap gap-1">
            <span v-for="scope in client.allowedScopes" :key="scope" class="badge badge-ghost badge-sm font-mono">{{ scope }}</span>
          </dd>
          <dt class="text-base-content/60">
            {{ t('clients.col_created') }}
          </dt>
          <dd>{{ created }}</dd>
        </dl>
      </div>
    </section>

    <section class="card border border-base-300 bg-base-100" data-testid="client-settings">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">
          {{ t('clients.settings_title') }}
        </h2>
        <template v-if="editable">
          <ClientForm
            v-model:fields="fields"
            mode="edit"
            :kinds="kinds"
            :uri-errors="uriErrors"
            :warnings="warnings"
            :name-error="nameError"
            :busy="busy"
            :error="error"
            :labels="formLabels"
            @edit="form.clearRow"
            @remove="form.removeRow"
            @submit="save"
          />
          <SavedNote v-if="done" :label="t('common.saved')" />
        </template>
        <dl v-else class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm" data-testid="client-settings-readonly">
          <dt class="text-base-content/60">
            {{ t('common.name') }}
          </dt>
          <dd>{{ client.name }}</dd>
          <dt class="text-base-content/60">
            {{ t('clients.redirect_uris') }}
          </dt>
          <dd>
            <ul class="space-y-0.5">
              <li v-for="uri in client.redirectUris" :key="uri">
                <code class="font-mono text-xs break-all">{{ uri }}</code>
              </li>
            </ul>
          </dd>
          <template v-if="client.postLogoutRedirectUris.length > 0">
            <dt class="text-base-content/60">
              {{ t('clients.post_logout_uris') }}
            </dt>
            <dd>
              <ul class="space-y-0.5">
                <li v-for="uri in client.postLogoutRedirectUris" :key="uri">
                  <code class="font-mono text-xs break-all">{{ uri }}</code>
                </li>
              </ul>
            </dd>
          </template>
          <dt class="text-base-content/60">
            {{ t('clients.refresh') }}
          </dt>
          <dd>{{ client.refreshTokens ? t('clients.refresh_on') : t('clients.refresh_off') }}</dd>
          <dt class="text-base-content/60">
            {{ t('clients.consent') }}
          </dt>
          <dd>{{ consentLabel }}</dd>
        </dl>
      </div>
    </section>

    <ClientSecrets
      v-if="kind.secret && !cimd"
      ref="secretsCard"
      :rows="rows"
      :can-edit="canEdit"
      :busy="secretBusy && !rotating && !revoking"
      :error="rotating || revoking ? null : secretError"
      :labels="secretsText"
      @generate="secrets.generate()"
      @rotate="secrets.askRotate()"
      @revoke="secrets.askRevoke"
    />

    <LinkList
      :rows="linkRows"
      :can-edit="editable"
      :loading="links.loading.value"
      :load-error="links.loadError.value"
      :labels="linkText"
      @link="links.askLink()"
      @edit="editLink"
      @unlink="unlinkRow"
    />

    <RotateSecretDialog
      v-model:grace="grace"
      :open="rotating"
      :options="options"
      :now-value="SecretGrace.NOW"
      :old-hint="oldHint"
      :busy="secretBusy"
      :error="secretError"
      :labels="rotateText"
      @confirm="secrets.confirmRotate()"
      @close="secrets.cancelRotate()"
    />

    <AppModal :open="!!revoking" :title="t('secrets.revoke_title')" @close="secrets.cancelRevoke()">
      <p class="text-sm">
        {{ t('secrets.revoke_old_body', { hint: revoking?.hint ?? '' }) }}
      </p>
      <ErrorText :error="secretError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="secretBusy" autofocus @click="secrets.cancelRevoke()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="secretBusy" @click="revoke">
          <span v-if="secretBusy" class="loading loading-spinner loading-xs" />
          {{ t('secrets.revoke_old') }}
        </button>
      </template>
    </AppModal>

    <ClientSecretDialog
      :open="!!revealed"
      :owner-id="client.id"
      :secret="revealed?.secret ?? ''"
      :labels="dialogText"
      @done="secretSaved"
    />

    <LinkDialog
      v-model:target-id="targetId"
      v-model:picked="picked"
      :open="!!editing"
      :mode="editing === 'edit' ? 'edit' : 'new'"
      :targets="targetOptions"
      :fixed-name="target?.name ?? ''"
      :scope-options="scopeOptions"
      :pick-error="pickError"
      :unverified="!!target?.unverified"
      :busy="linkBusy"
      :error="linkError"
      :labels="linkDialogText"
      @target="links.pickTarget"
      @confirm="links.confirmEdit()"
      @close="links.cancelEdit()"
    />

    <AppModal :open="!!unlinking" :title="t('links.unlink_title', { name: unlinkingName })" @close="links.cancelUnlink()">
      <p class="text-sm">
        {{ t('links.unlink_body', { client: client.name, rs: unlinkingName }) }}
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

    <AppModal :open="page.deleting.value" :title="cimd ? t('rs.cimd_revoke_title', { name: client.name }) : t('clients.delete_title', { name: client.name })" @close="page.cancelDelete()">
      <p class="text-sm">
        {{ cimd ? t('rs.cimd_revoke_body', { name: client.name }) : t('clients.delete_body') }}
      </p>
      <ErrorText :error="page.error.value" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="page.busy.value" autofocus @click="page.cancelDelete()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="page.busy.value" @click="remove">
          <span v-if="page.busy.value" class="loading loading-spinner loading-xs" />
          {{ cimd ? t('rs.cimd_revoke') : t('common.delete') }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
