import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ClientList from './ClientList.vue';

const rows = [
  { id: '7Hq2mXv9LpTzR4kWnB8sY', name: 'Acme Portal', type: 'Web', icon: 'lucide:app-window', host: 'portal.acme.example', lastUsed: '2 min ago', to: '#' },
  { id: 'b9FsJe4UoA6iGrX1tNmPc', name: 'Field App', type: 'Native', icon: 'lucide:monitor-smartphone', host: 'com.acme.field', lastUsed: 'Yesterday', to: '#' },
  { id: 'Zt3nQw8RkV1sLyH6cXgFm', name: 'Fieldnote', type: 'MCP', icon: 'lucide:plug', host: '127.0.0.1', lastUsed: 'Never', to: '#' },
  { id: 'Lq3vT8nWx2KpZr6YbM9cD', name: 'Othra console', type: 'Web', icon: 'lucide:app-window', host: 'othra.example', lastUsed: '2 days ago', to: '#', system: true },
];
const labels = { caption: 'Clients', name: 'Name', type: 'Type', clientId: 'Client ID', host: 'Redirect host', lastUsed: 'Last used', actions: 'Actions', moreFor: 'More actions for {name}', open: 'Open', copyId: 'Copy client ID', copied: 'Copied', system: 'System' };

const meta = {
  title: 'Clients/ClientList',
  component: ClientList,
  args: { rows, labels },
} satisfies Meta<typeof ClientList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ClientList },
  setup: () => ({ args }),
  template: '<div class="p-4"><ClientList v-bind="args" /></div>',
});

export const Default: Story = { name: 'Clients (artifact)', render };
export const Loading: Story = { args: { rows: [], labels, loading: true }, render };
export const Mobile: Story = {
  render: args => ({
    components: { ClientList },
    setup: () => ({ args }),
    template: '<div class="w-[358px] p-4"><ClientList v-bind="args" /></div>',
  }),
};
