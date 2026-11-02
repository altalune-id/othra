import type { Meta, StoryObj } from '@storybook/vue3-vite';
import KeyValue from './KeyValue.vue';
import Section from './Section.vue';
import StatusLabel from './StatusLabel.vue';

const meta = {
  title: 'UI/Section',
  component: Section,
  args: { title: 'Key rotation' },
} satisfies Meta<typeof Section>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: args => ({
    components: { Section },
    setup: () => ({ args }),
    template: `<Section v-bind="args"><p class="text-ui">Retired Aug 15, active since Aug 15.</p></Section>`,
  }),
};

const status = [{ label: 'Discovery' }, { label: 'Signing keys' }, { label: 'Active key' }, { label: 'Sign-in methods' }];

export const IssuerStatus: Story = {
  name: 'Issuer status rows (artifact)',
  render: () => ({
    components: { KeyValue, Section, StatusLabel },
    setup: () => ({ status }),
    template: `
      <Section title="Issuer status" flush class="max-w-2xl">
        <KeyValue :items="status">
          <template #value-0><StatusLabel tone="success" label="Serving"><span class="font-mono text-xs">/.well-known/openid-configuration</span> 200, checked 2 min ago</StatusLabel></template>
          <template #value-1><StatusLabel tone="success" label="2 published" detail="JWKS reachable" /></template>
          <template #value-2><StatusLabel tone="success" label="2026-08-a" detail="ES256, 78 days old" /></template>
          <template #value-3><StatusLabel tone="success" label="Email link, Google"><a class="link" href="#">See the hosted page</a></StatusLabel></template>
        </KeyValue>
      </Section>`,
  }),
};

export const WithLink: Story = {
  name: 'Header with link and subtitle (artifact)',
  render: () => ({
    components: { Section },
    template: `
      <Section title="Setup" description="4 of 6 done" flush class="max-w-sm">
        <template #actions><a class="link text-ui" href="#">Audit log</a></template>
        <ul class="divide-y divide-base-300 text-ui"><li class="px-4 py-2">Set the issuer handle</li><li class="px-4 py-2">Choose sign-in methods</li></ul>
      </Section>`,
  }),
};

export const Danger: Story = {
  args: { title: 'Delete project', description: 'Deletes every client, key and end user in this project.', tone: 'danger' },
};
