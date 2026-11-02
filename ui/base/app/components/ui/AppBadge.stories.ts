import type { Meta, StoryObj } from '@storybook/vue3-vite';
import AppBadge from './AppBadge.vue';
import StatusLabel from './StatusLabel.vue';

const meta = {
  title: 'UI/Badges',
  component: AppBadge,
  args: { label: 'Web', icon: 'lucide:globe' },
} satisfies Meta<typeof AppBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Playground: Story = {};

export const ClientTypes: Story = {
  name: 'Client types (artifact)',
  render: () => ({
    components: { AppBadge },
    template: `
      <div class="flex flex-wrap gap-2">
        <AppBadge label="Web" icon="lucide:globe" />
        <AppBadge label="SPA" icon="lucide:app-window" />
        <AppBadge label="Native" icon="lucide:terminal" />
        <AppBadge label="Machine" icon="lucide:server" />
        <AppBadge label="MCP" icon="lucide:plug" />
        <AppBadge label="Unverified" icon="lucide:shield-alert" outline />
      </div>`,
  }),
};

export const Environments: Story = {
  name: 'Environments (artifact)',
  render: () => ({
    components: { AppBadge },
    template: `
      <ul class="menu menu-sm w-56 p-0 text-ui">
        <li><button type="button"><span class="font-mono">acme-prod</span><AppBadge label="Production" size="xs" /></button></li>
        <li><button type="button"><span class="font-mono">acme-labs</span><AppBadge label="Labs" size="xs" /></button></li>
      </ul>`,
  }),
};

export const Status: Story = {
  name: 'Status dot + label (artifact)',
  render: () => ({
    components: { StatusLabel },
    template: `
      <div class="flex flex-col gap-2">
        <StatusLabel tone="success" label="Serving"><span class="font-mono text-xs">/.well-known/openid-configuration</span> 200, checked 2 min ago</StatusLabel>
        <StatusLabel tone="success" label="2 published" detail="JWKS reachable" />
        <StatusLabel tone="warning" label="Rotates in 6 days" />
        <StatusLabel tone="error" label="Not reachable" detail="Retry" />
      </div>`,
  }),
};
