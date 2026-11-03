<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import PageHeader from '#base/components/ui/PageHeader.vue';
import SecretReveal from '#base/components/ui/SecretReveal.vue';
import { useToast } from '#base/components/ui/toast';
import ClientForm from '~/components/client/ClientForm.vue';
import ClientKindPicker from '~/components/client/ClientKindPicker.vue';
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

const typeDone = ref(false);
const stored = ref(false);
const secretLabels = computed(() => ({
  once: t('secret.body'),
  copy: t('secret.copy_secret'),
  copyEnv: t('clients.copy_env'),
  confirm: t('secret.confirm'),
  missing: secretError.value?.message ?? t('error.generate_secret'),
}));
const secretDone = ref(false);
const withSecret = computed(() => kind.value.secret);
const step = computed(() => {
  if (!done.value || !result.value) {
    return typeDone.value ? 'details' : 'type';
  }
  return withSecret.value && !secretDone.value ? 'secret' : 'quickstart';
});
const steps = computed(() => [
  { key: 'type', label: t('clients.step_type') },
  { key: 'details', label: t('clients.step_details') },
  ...(withSecret.value ? [{ key: 'secret', label: t('clients.step_secret') }] : []),
  { key: 'quickstart', label: t('clients.step_quickstart') },
]);
const heading = computed(() => {
  switch (step.value) {
    case 'type':
      return { title: t('clients.type_title'), description: t('clients.type_subtitle') };
    case 'details':
      return { title: t('clients.details_title'), description: kinds.value.find(k => k.value === fields.value.kind)?.rec ?? '' };
    case 'secret':
      return { title: t('secret.title'), description: t('clients.secret_subtitle') };
    default:
      return { title: t('clients.quickstart_title', { name: result.value?.name ?? '' }), description: t('clients.quickstart_body') };
  }
});
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

const toast = useToast();

onBeforeRouteLeave(() => {
  if (!holding.value) {
    return true;
  }
  toast.push('info', t('secret.leave_first'));
  return false;
});
onMounted(() => window.addEventListener('beforeunload', warnLeave));
onBeforeUnmount(() => window.removeEventListener('beforeunload', warnLeave));

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('clients.new')} · Othra` });
</script>

<template>
  <div class="flex max-w-3xl min-w-0 flex-col gap-4">
    <ul class="steps steps-horizontal w-full max-w-xl text-xs">
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
    <PageHeader :title="heading.title" :description="heading.description" />

    <div v-if="step === 'type'" class="flex flex-col gap-4" data-testid="type-step">
      <ClientKindPicker
        :kinds="kinds"
        :model-value="fields.kind"
        :legend="t('clients.kind')"
        :note="t('clients.kind_fixed')"
        @update:model-value="form.setKind"
      />
      <div class="flex gap-2">
        <AppButton variant="plain" size="sm" :to="base">
          {{ t('common.cancel') }}
        </AppButton>
        <AppButton size="sm" @click="typeDone = true">
          {{ t('clients.continue_details') }}
        </AppButton>
      </div>
    </div>

    <div v-else-if="step === 'details'" class="flex max-w-xl flex-col gap-4">
      <p class="flex flex-wrap items-center gap-2 text-ui">
        <span>{{ t('clients.type_line', { kind: kinds.find(k => k.value === fields.kind)?.label ?? '' }) }}</span>
        <button type="button" class="link text-ui" :disabled="busy" @click="typeDone = false">
          {{ t('clients.change_type') }}
        </button>
      </p>
      <ClientForm
        v-model:fields="fields"
        :kind-chosen="true"
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
        @back="typeDone = false"
      />
    </div>

    <div v-else-if="step === 'secret'" class="flex max-w-xl flex-col gap-4" data-testid="secret-step">
      <SecretReveal
        v-model:confirmed="stored"
        :value="revealed?.secret ?? ''"
        :labels="secretLabels"
        :env="revealed ? { OIDC_CLIENT_SECRET: revealed.secret } : {}"
      >
        <template #retry>
          <AppButton variant="plain" size="sm" :busy="busy" @click="form.retrySecret()">
            {{ t('clients.generate_again') }}
          </AppButton>
        </template>
      </SecretReveal>
      <div>
        <AppButton size="sm" :disabled="!revealed || !stored" @click="secretDone = true">
          {{ t('clients.continue_quickstart') }}
        </AppButton>
      </div>
    </div>

    <ClientQuickstart
      v-else
      :values="values"
      :secret="withSecret"
      :open-to="`${base}/${result?.id}`"
      :back-to="base"
    />
  </div>
</template>
