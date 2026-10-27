<script setup lang="ts">
import { ref, watch } from 'vue';
import AppModal from '#base/components/ui/AppModal.vue';
import CopyButton from '#base/components/ui/CopyButton.vue';
import SecretField from '#base/components/ui/SecretField.vue';

export interface ClientSecretDialogLabels {
  title: string;
  body: string;
  ownerId: string;
  secret: string;
  copyOwnerId: string;
  copySecret: string;
  confirm: string;
  closeDisabled: string;
  done: string;
}

const { open, ownerId, secret, labels = {
  title: 'Store this secret now',
  body: 'This is the only time it is shown. Put it in your app\'s config. If you lose it, rotate the secret.',
  ownerId: 'Client ID',
  secret: 'Client secret',
  copyOwnerId: 'Copy client ID',
  copySecret: 'Copy client secret',
  confirm: 'I\'ve stored this secret somewhere safe',
  closeDisabled: 'Tick the box above to close.',
  done: 'Done',
} } = defineProps<{ open: boolean, ownerId: string, secret: string, labels?: ClientSecretDialogLabels }>();

const emit = defineEmits<{ done: [] }>();
const stored = ref(false);

watch(() => open, () => {
  stored.value = false;
});

function done() {
  if (stored.value) {
    emit('done');
  }
}
</script>

<template>
  <AppModal :open="open" :title="labels.title" :dismissible="stored" @close="done">
    <div class="space-y-3">
      <p class="flex items-start gap-1.5 text-sm text-base-content">
        <Icon name="lucide:triangle-alert" size="16" class="mt-0.5 shrink-0 text-warning" />
        {{ labels.body }}
      </p>
      <div class="space-y-1">
        <span class="block text-xs font-medium text-base-content/70">{{ labels.ownerId }}</span>
        <SecretField :value="ownerId" :label="labels.copyOwnerId" />
      </div>
      <div class="space-y-1">
        <span class="block text-xs font-medium text-base-content/70">{{ labels.secret }}</span>
        <div class="flex items-center gap-2 rounded-box border border-base-300 bg-base-200 py-1.5 pr-1.5 pl-3">
          <code class="min-w-0 flex-1 font-mono text-xs leading-relaxed break-all" data-testid="client-secret">{{ secret }}</code>
          <CopyButton :value="secret" :label="labels.copySecret" class="shrink-0" autofocus />
        </div>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="stored" type="checkbox" class="checkbox checkbox-sm rounded-sm">
        {{ labels.confirm }}
      </label>
    </div>
    <template #footer>
      <span v-if="!stored" class="mr-auto self-center text-xs text-base-content/60">{{ labels.closeDisabled }}</span>
      <button type="button" class="btn btn-primary btn-sm" :disabled="!stored" @click="done">
        {{ labels.done }}
      </button>
    </template>
  </AppModal>
</template>
