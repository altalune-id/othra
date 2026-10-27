<script setup lang="ts">
import AppModal from '#base/components/ui/AppModal.vue';
import ErrorText from '#base/components/ui/ErrorText.vue';
import ProjectReadOnlyNote from '~/components/project/ProjectReadOnlyNote.vue';
import GoogleUpstreamCard from '~/components/project/signin/GoogleUpstreamCard.vue';
import { googleUpstreamCardLabels } from '~/components/project/signin/labels';
import { useSignIn } from '~/components/project/signin/useSignIn';
import { useProjectGuard } from '~/components/project/useProjectAccess';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const route = useRoute();
const { t } = useI18n();
const orgs = useOrgsStore();
const projects = useProjectsStore();

const org = computed(() => orgs.bySlug(route.params.org as string));
const listed = computed(() => projects.items.find(p => p.orgId === org.value?.id && p.slug === route.params.project));
if (!listed.value) {
  throw createError({ statusCode: 404, statusMessage: t('error.project_not_found'), fatal: true });
}

const signIn = useSignIn(listed.value.id);
const { google, loadError, settingUp, clientId, secret, replacingSecret, busy, saved, error, toggleError, emailError, removing, removeError, emailLink, emailLocked, emailLockReason, googleLocked, removeLocked, redirectUri, fieldErrors } = signIn;
const labels = computed(() => googleUpstreamCardLabels(t));
const ids = { email: useId(), emailHelp: useId(), emailLocked: useId() };

function onEmailToggle(event: Event) {
  const input = event.target as HTMLInputElement;
  const wanted = input.checked;
  input.checked = emailLink.value;
  return signIn.setEmailLink(wanted);
}

const guard = useProjectGuard(() => listed.value);
const canEdit = computed(() => guard.value.canWrite);
const emailLockedOn = computed(() => emailLocked.value || guard.value.system);
const emailLockText = computed(() => {
  if (guard.value.system) {
    return t('signin.email_locked_system');
  }
  return emailLockReason.value === 'unlinked' ? t('signin.email_locked_unlinked') : t('signin.email_locked');
});
await Promise.all([signIn.load(), useAuthStore().load()]);

useHead({ title: () => `${listed.value?.name ?? t('project.title')} · ${t('signin.title')} · Othra` });
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-bold">
        {{ t('signin.title') }}
      </h1>
      <p class="text-sm text-base-content/60">
        {{ t('signin.subtitle') }}
      </p>
    </div>
    <ProjectReadOnlyNote v-if="guard.system && guard.canWrite" icon="lucide:shield" :text="t('project.system_note')" />
    <ProjectReadOnlyNote v-else-if="!canEdit" />
    <ErrorText :error="loadError" />

    <section class="card border border-base-300 bg-base-100" data-testid="email-link-card">
      <div class="card-body gap-2">
        <template v-if="canEdit">
          <label :for="ids.email" class="flex items-center gap-3 text-sm font-medium">
            <input
              :id="ids.email"
              :checked="emailLink"
              type="checkbox"
              class="toggle toggle-sm"
              :class="{ 'cursor-not-allowed': emailLockedOn }"
              :disabled="busy"
              :aria-disabled="emailLockedOn"
              :aria-describedby="emailLockedOn ? `${ids.emailHelp} ${ids.emailLocked}` : ids.emailHelp"
              @click="emailLockedOn && $event.preventDefault()"
              @change="onEmailToggle"
            >
            {{ t('signin.email_link') }}
          </label>
          <p :id="ids.emailHelp" class="label whitespace-normal">
            {{ t('signin.email_link_help') }}
          </p>
          <p v-if="emailLockedOn" :id="ids.emailLocked" class="flex items-start gap-1 text-xs text-base-content">
            <Icon name="lucide:lock" size="14" class="shrink-0 text-info" />
            {{ emailLockText }}
          </p>
          <ErrorText :error="emailError" />
        </template>
        <dl v-else class="space-y-1 text-sm">
          <dt class="fieldset-legend">
            {{ t('signin.email_link') }}
          </dt>
          <dd>{{ emailLink ? t('signin.on') : t('signin.off') }}</dd>
        </dl>
      </div>
    </section>

    <GoogleUpstreamCard
      v-model:client-id="clientId"
      v-model:secret="secret"
      v-model:replacing-secret="replacingSecret"
      :can-edit="canEdit"
      :google="google"
      :setting-up="settingUp"
      :redirect-uri="redirectUri"
      :busy="busy"
      :saved="saved"
      :error="error"
      :toggle-error="toggleError"
      :field-errors="fieldErrors"
      :locked="googleLocked"
      :remove-locked="removeLocked"
      :labels="labels"
      @setup="signIn.startSetup()"
      @cancel="signIn.cancelSetup()"
      @save="signIn.save()"
      @toggle="signIn.setGoogleEnabled($event)"
      @remove="signIn.askRemove()"
    />

    <AppModal :open="removing" :title="t('signin.google_remove_title')" @close="signIn.cancelRemove()">
      <p class="text-sm">
        {{ t('signin.google_remove_body') }}
      </p>
      <ErrorText :error="removeError" />
      <template #footer>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="signIn.cancelRemove()">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-error btn-sm" :disabled="busy" @click="signIn.confirmRemove()">
          {{ t('signin.google_remove') }}
        </button>
      </template>
    </AppModal>
  </div>
</template>
