<script setup lang="ts">
import type { SigningKey } from '#gen/keys/v1/keys_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import Section from '#base/components/ui/Section.vue';
import { formatDate } from '#base/lib/format/date';
import { SigningKeyStatus } from '#gen/keys/v1/keys_pb';
import { shortKid } from '~/components/project/keys/useSigningKeys';

const { keys } = defineProps<{ keys: SigningKey[] }>();
const { t, locale } = useI18n();

const day = (ts?: SigningKey['createdAt']) => (ts ? formatDate(timestampDate(ts), locale.value) : '');
const steps = computed(() => keys
  .filter(k => k.status === SigningKeyStatus.RETIRED || k.status === SigningKeyStatus.RETIRING || k.status === SigningKeyStatus.ACTIVE)
  .sort((a, b) => (a.createdAt ? timestampDate(a.createdAt).getTime() : 0) - (b.createdAt ? timestampDate(b.createdAt).getTime() : 0))
  .map(k => ({
    kid: shortKid(k.kid),
    active: k.status === SigningKeyStatus.ACTIVE,
    when: k.status === SigningKeyStatus.ACTIVE
      ? t('overview.rotation_active', { date: day(k.activatedAt ?? k.createdAt) })
      : t(k.status === SigningKeyStatus.RETIRED ? 'overview.rotation_retired' : 'overview.rotation_retiring', { date: day(k.retireAfter) }),
  })));
</script>

<template>
  <Section v-if="steps.length" :title="t('overview.rotation_title')" data-testid="key-rotation">
    <div class="overflow-x-auto">
      <ul class="timeline max-sm:timeline-vertical max-sm:timeline-compact sm:timeline-horizontal w-full *:sm:flex-1 text-ui">
        <li v-for="(step, i) in steps" :key="step.kid">
          <hr v-if="i > 0" :class="{ 'bg-primary': step.active }">
          <div class="timeline-start text-xs text-base-content/70">
            {{ step.when }}
          </div>
          <div class="timeline-middle" :class="{ 'text-primary': step.active }">
            <Icon :name="step.active ? 'lucide:circle-dot' : 'lucide:circle'" :size="step.active ? 16 : 14" class="shrink-0" aria-hidden="true" />
          </div>
          <div class="timeline-end timeline-box font-mono text-xs">
            {{ step.kid }}
          </div>
          <hr v-if="i < steps.length - 1">
        </li>
      </ul>
    </div>
  </Section>
</template>
