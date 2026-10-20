<script setup lang="ts">
import type { ShownError } from '../../lib/apperror/message';
import type { VerificationName } from './verification';
import { ref, useId } from 'vue';
import CopyButton from '../ui/CopyButton.vue';
import ErrorText from '../ui/ErrorText.vue';
import UrlField from '../ui/UrlField.vue';
import VerificationStatus from './VerificationStatus.vue';

export interface VerificationCardLabels {
  title: string;
  help: string;
  metadataUrl: string;
  lastCheck: string;
  verifiedAt: string;
  mustInclude: string;
  copySnippet: string;
  verifyNow: string;
  verifying: string;
  trust: string;
  trustHelp: string;
  untrustHelp: string;
  httpLocked: string;
  copyLabel: string;
  openLabel: string;
}

const props = withDefaults(defineProps<{
  status: VerificationName,
  statusLabel: string,
  body: string,
  metadataUrl: string,
  lastCheck?: string,
  verifiedAt?: string,
  failure?: string,
  recheck?: string,
  snippet?: string,
  canEdit?: boolean,
  trustAvailable?: boolean,
  http?: boolean,
  verifying?: boolean,
  verifyError?: ShownError | null,
  passed?: string,
  trustBusy?: boolean,
  trustError?: ShownError | null,
  labels?: VerificationCardLabels,
}>(), {
  lastCheck: '',
  verifiedAt: '',
  failure: '',
  recheck: '',
  snippet: '',
  canEdit: false,
  trustAvailable: false,
  http: false,
  verifying: false,
  verifyError: null,
  passed: '',
  trustBusy: false,
  trustError: null,
  labels: () => ({
    title: 'Verification',
    help: 'Othra reads your app\'s metadata at this URL. It must list this identifier as resource and this issuer in authorization_servers.',
    metadataUrl: 'Metadata URL',
    lastCheck: 'Last check',
    verifiedAt: 'Verified',
    mustInclude: 'Your metadata must include:',
    copySnippet: 'Copy metadata snippet',
    verifyNow: 'Verify now',
    verifying: 'Checking…',
    trust: 'Trust without verification',
    trustHelp: 'For internal servers Othra can\'t reach. Othra won\'t check it again.',
    untrustHelp: 'Turning it off sends it back to Pending. Apps can\'t get tokens for it until it is verified.',
    httpLocked: 'An http identifier can\'t be verified, so trust stays on.',
    copyLabel: 'Copy {label}',
    openLabel: 'Open {label} in a new tab',
  }),
});

const emit = defineEmits<{ verify: [], toggleTrust: [] }>();
const verifyButton = ref<HTMLButtonElement>();

defineExpose({ focusVerify: () => verifyButton.value?.focus() });
const trustId = useId();
const trusted = () => props.status === 'trusted';
</script>

<template>
  <section class="card border border-base-300 bg-base-100" data-testid="verification-card">
    <div class="card-body gap-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="card-title text-base">
          {{ labels.title }}
        </h2>
        <VerificationStatus :name="status" :label="statusLabel" />
      </div>
      <p class="text-sm text-base-content">
        {{ body }}
      </p>
      <p class="text-sm text-base-content/70">
        {{ labels.help }}
      </p>
      <UrlField v-if="metadataUrl" :label="labels.metadataUrl" :value="metadataUrl" size="sm" open :copy-label="labels.copyLabel" :open-label="labels.openLabel" />
      <dl v-if="lastCheck || verifiedAt" class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <template v-if="lastCheck">
          <dt class="text-base-content/60">
            {{ labels.lastCheck }}
          </dt>
          <dd>{{ lastCheck }}</dd>
        </template>
        <template v-if="verifiedAt">
          <dt class="text-base-content/60">
            {{ labels.verifiedAt }}
          </dt>
          <dd>{{ verifiedAt }}</dd>
        </template>
      </dl>
      <div v-if="recheck" role="status" class="alert alert-soft">
        <Icon name="lucide:info" size="18" class="text-info" />
        <span class="text-sm text-base-content">{{ recheck }}</span>
      </div>
      <div v-else-if="failure" role="status" class="alert alert-soft">
        <Icon name="lucide:circle-alert" size="18" class="text-error" />
        <span class="text-sm break-words text-base-content">{{ failure }}</span>
      </div>
      <div v-if="snippet && !trusted()" class="space-y-1">
        <span class="block text-xs font-medium text-base-content/70">{{ labels.mustInclude }}</span>
        <div class="flex items-start gap-2 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
          <pre class="min-w-0 flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{{ snippet }}</pre>
          <CopyButton :value="snippet" :label="labels.copySnippet" class="shrink-0" />
        </div>
      </div>
      <template v-if="canEdit && !trusted()">
        <div>
          <button ref="verifyButton" type="button" class="btn btn-sm max-sm:btn-block" :disabled="verifying" @click="emit('verify')">
            <span v-if="verifying" class="loading loading-spinner loading-xs" />
            <Icon v-else name="lucide:refresh-cw" size="16" />
            {{ verifying ? labels.verifying : labels.verifyNow }}
          </button>
        </div>
        <p v-if="passed" role="status" class="flex items-start gap-1.5 text-sm text-base-content">
          <Icon name="lucide:circle-check" size="16" class="mt-0.5 shrink-0 text-success" />
          {{ passed }}
        </p>
        <ErrorText :error="verifyError" />
      </template>
      <template v-if="trustAvailable && (canEdit || trusted())">
        <div class="divider my-0" />
        <div v-if="canEdit" class="space-y-1">
          <label :for="trustId" class="flex items-center gap-3 text-sm font-medium">
            <input
              :id="trustId"
              type="checkbox"
              class="toggle toggle-warning toggle-sm"
              :checked="trusted()"
              :disabled="trustBusy || (http && trusted())"
              :aria-describedby="`${trustId}-help`"
              @click.prevent="emit('toggleTrust')"
            >
            {{ labels.trust }}
          </label>
          <p :id="`${trustId}-help`" class="label whitespace-normal">
            {{ http && trusted() ? labels.httpLocked : trusted() ? labels.untrustHelp : labels.trustHelp }}
          </p>
          <ErrorText :error="trustError" />
        </div>
        <p v-else-if="trusted()" class="flex items-center gap-1.5 text-sm text-base-content">
          <Icon name="lucide:shield-alert" size="16" class="text-warning" />
          {{ labels.trust }}
        </p>
      </template>
    </div>
  </section>
</template>
