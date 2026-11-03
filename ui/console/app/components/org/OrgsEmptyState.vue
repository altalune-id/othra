<script setup lang="ts">
import AppButton from '#base/components/ui/AppButton.vue';
import EmptyState from '#base/components/ui/EmptyState.vue';

export interface OrgsEmptyStateLabels {
  title: string;
  body: string;
  create: string;
  signOut: string;
}

const { canCreate, logoutUrl, labels } = defineProps<{ canCreate: boolean, logoutUrl: string, labels: OrgsEmptyStateLabels }>();
const emit = defineEmits<{ create: [] }>();
</script>

<template>
  <EmptyState icon="lucide:building-2" :title="labels.title" :body="labels.body">
    <template #actions>
      <AppButton v-if="canCreate" size="sm" @click="emit('create')">
        <Icon name="lucide:plus" size="16" class="shrink-0" aria-hidden="true" />
        {{ labels.create }}
      </AppButton>
      <form method="post" :action="logoutUrl">
        <AppButton type="submit" variant="ghost" size="sm">
          <Icon name="lucide:log-out" size="16" class="shrink-0" aria-hidden="true" />
          {{ labels.signOut }}
        </AppButton>
      </form>
    </template>
  </EmptyState>
</template>
