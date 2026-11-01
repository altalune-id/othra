<script setup lang="ts">
import PageHeader from '#base/components/ui/PageHeader.vue';
import SecretReveal from '#base/components/ui/SecretReveal.vue';
import ClientForm from '~/components/client/ClientForm.vue';
import ClientQuickstart from '~/components/client/ClientQuickstart.vue';
import { clientFormLabels, kindOptions } from '~/components/client/labels';
import { useClientForm } from '~/components/client/useClientForm';
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

if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}
const projectId = listed.value.id;
const base = computed(() => `/o/${route.params.org}/projects/${route.params.project}/clients`);
if (!canEdit.value) {
  await navigateTo(base.value, { replace: true });
}
void detail.ensureLoaded(projectId);

const form = useClientForm(projectId);
const { fields, kind, warnings, uriErrors, nameError, busy, error, done, result, secretError, revealed } = form;
const labels = computed(() => clientFormLabels(t, 'create'));
const kinds = computed(() => kindOptions(t));

const stored = ref(false);
const secretDone = ref(false);
const withSecret = computed(() => kind.value.secret);
const step = computed(() => {
  if (!done.value || !result.value) {
    return 'details';
  }
  return withSecret.value && !secretDone.value ? 'secret' : 'quickstart';
});
const steps = computed(() => [
  { key: 'details', label: t('clients.step_details') },
  ...(withSecret.value ? [{ key: 'secret', label: t('clients.step_secret') }] : []),
  { key: 'quickstart', label: t('clients.step_quickstart') },
]);
const stepIndex = computed(() => steps.value.findIndex(s => s.key === step.value));
const values = computed(() => ({
  issuer: detail.get(projectId)?.issuerUrl ?? '',
  clientId: result.value?.id ?? '',
  redirectUri: result.value?.redirectUris[0] ?? '',
  secret: revealed.value?.secret,
}));

async function submit() {
  const client = await form.submit();
  if (client && detail.get(projectId)) {
    void detail.load(projectId);
  }
}

const holding = computed(() => step.value === 'secret' && !!revealed.value);

function warnLeave(event: BeforeUnloadEvent) {
  if (holding.value) {
    event.preventDefault();
  }
}

onBeforeRouteLeave(() => !holding.value);
onMounted(() => window.addEventListener('beforeunload', warnLeave));
onBeforeUnmount(() => window.removeEventListener('beforeunload', warnLeave));

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('clients.new')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <PageHeader :title="result?.name || t('clients.new')" :crumbs="[{ label: t('clients.title'), to: base }, { label: t('clients.new') }]" :crumbs-label="t('nav.breadcrumbs')" />
    <ul class="steps w-full text-xs">
      <li
        v-for="(s, i) in steps"
        :key="s.key"
        class="step"
        :class="{ 'step-primary': i <= stepIndex }"
        :aria-current="i === stepIndex ? 'step' : undefined"
      >
        {{ s.label }}
      </li>
    </ul>
    <p class="sr-only">
      {{ t('clients.step_of', { n: stepIndex + 1, total: steps.length }) }}
    </p>

    <section v-if="step === 'details'" class="card border border-base-300 bg-base-100">
      <div class="card-body">
        <ClientForm
          v-model:fields="fields"
          mode="create"
          :kinds="kinds"
          :uri-errors="uriErrors"
          :warnings="warnings"
          :name-error="nameError"
          :busy="busy"
          :error="error"
          :labels="labels"
          @kind="form.setKind"
          @edit="form.clearRow"
          @remove="form.removeRow"
          @submit="submit"
          @cancel="navigateTo(base)"
        />
      </div>
    </section>

    <section v-else-if="step === 'secret'" class="card border border-base-300 bg-base-100" data-testid="secret-step">
      <div class="card-body gap-4">
        <h2 class="card-title text-base">
          {{ t('secret.title') }}
        </h2>
        <SecretReveal
          :value="revealed?.secret ?? ''"
          :label="t('secret.copy_secret')"
          :warning="t('secret.body')"
          :confirm-label="t('secret.confirm')"
          env-name="OTHRA_CLIENT_SECRET"
          :env-label="t('clients.copy_env')"
          :error="revealed ? null : secretError"
          @confirmed="stored = $event"
        >
          <template #retry>
            <button type="button" class="btn btn-sm" :disabled="busy" @click="form.retrySecret()">
              <span v-if="busy" class="loading loading-spinner loading-xs" aria-hidden="true" />
              {{ t('clients.generate_again') }}
            </button>
          </template>
        </SecretReveal>
        <div class="flex justify-end">
          <button type="button" class="btn btn-primary btn-sm" :disabled="!revealed || !stored" @click="secretDone = true">
            {{ t('clients.continue_quickstart') }}
          </button>
        </div>
      </div>
    </section>

    <ClientQuickstart
      v-else
      :values="values"
      :name="result?.name ?? ''"
      :open-to="`${base}/${result?.id}`"
      :back-to="base"
    />
  </div>
</template>
