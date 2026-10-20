import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ResourceServerList from './ResourceServerList.vue';

const rows = [
  { id: 'a', name: 'Storefront API', identifier: 'https://api.shop.example.com', status: 'verified', statusLabel: 'Verified', checked: '2 hours ago', checkedTitle: 'Oct 20, 2026, 12:00 PM', to: '#' },
  { id: 'b', name: 'Storefront MCP', identifier: 'https://shop.example.com/mcp', status: 'failed', statusLabel: 'Failed', checked: '5 minutes ago', checkedTitle: 'Oct 20, 2026, 1:55 PM', to: '#' },
  { id: 'c', name: 'Local tools', identifier: 'http://127.0.0.1:8080/mcp', status: 'trusted', statusLabel: 'Trusted', checked: '', checkedTitle: '', to: '#' },
  { id: 'd', name: 'Billing API', identifier: 'https://billing.example.com', status: 'pending', statusLabel: 'Pending', checked: '', checkedTitle: '', to: '#' },
] as const;

const meta = {
  title: 'Resource servers/ResourceServerList',
  component: ResourceServerList,
  args: { rows: [...rows] },
} satisfies Meta<typeof ResourceServerList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ResourceServerList },
  setup: () => ({ args }),
  template: '<div class="max-w-4xl p-8"><div class="card border border-base-300 bg-base-100"><div class="card-body"><ResourceServerList v-bind="args" /></div></div></div>',
});

export const AllStates: Story = { render };
export const Loading: Story = { args: { rows: [], loading: true }, render };
export const Mobile: Story = {
  render: args => ({
    components: { ResourceServerList },
    setup: () => ({ args }),
    template: '<div class="w-[358px] p-4"><ResourceServerList v-bind="args" /></div>',
  }),
};
