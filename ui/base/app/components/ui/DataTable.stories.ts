import type { Meta, StoryObj } from '@storybook/vue3-vite';
import type { Column } from './types';
import AppBadge from './AppBadge.vue';
import AppButton from './AppButton.vue';
import DataTable from './DataTable.vue';
import EmptyState from './EmptyState.vue';

const meta = {
  title: 'UI/DataTable',
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

const columns: Column[] = [
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type' },
  { key: 'id', label: 'Client ID', mono: true },
  { key: 'host', label: 'Redirect host', mono: true },
  { key: 'used', label: 'Last used' },
  { key: 'menu', label: 'Actions', hideLabel: true, align: 'end' },
];
const rows = [
  { id: '7Hq2mXv9LpTzR4kWnB8sY', name: 'Acme Portal', type: 'Web', icon: 'lucide:app-window', host: 'portal.acme.example', used: '2 min ago' },
  { id: 'Rp8cWx2LhM5oTzK7vYqDn', name: 'Support Console', type: 'SPA', icon: 'lucide:panels-top-left', host: 'support.acme.example', used: 'Yesterday' },
  { id: 'b9FsJe4UoA6iGrX1tNmPc', name: 'Field App', type: 'Native', icon: 'lucide:smartphone', host: 'com.acme.field', used: 'Yesterday' },
  { id: 'Zt3nQw8RkV1sLyH6cXgFm', name: 'Billing Sync', type: 'Machine', icon: 'lucide:cpu', host: 'No redirect', used: '3 days ago' },
];

const clients = `
  <DataTable caption="Clients" row-key="id" :columns="columns" :rows="rows">
    <template #cell-name="{ row }"><a class="link link-hover font-medium" href="#">{{ row.name }}</a></template>
    <template #cell-type="{ row }"><AppBadge :label="row.type" :icon="row.icon" /></template>
    <template #cell-id="{ row }"><span class="block max-w-44 truncate" :title="row.id">{{ row.id }}</span></template>
    <template #cell-used="{ row }"><span class="tabular-nums text-base-content/70">{{ row.used }}</span></template>
    <template #cell-menu="{ row }"><AppButton variant="ghost" size="xs" square :aria-label="'More actions for ' + row.name"><Icon name="lucide:ellipsis" size="14" aria-hidden="true" /></AppButton></template>
  </DataTable>`;

export const Clients: Story = {
  name: 'Clients (artifact)',
  render: () => ({
    components: { AppBadge, AppButton, DataTable },
    setup: () => ({ columns, rows }),
    template: clients,
  }),
};

export const Loading: Story = {
  render: () => ({
    components: { DataTable },
    setup: () => ({ columns }),
    template: '<DataTable caption="Clients" row-key="id" :columns="columns" :rows="[]" loading />',
  }),
};

export const Empty: Story = {
  render: () => ({
    components: { DataTable, EmptyState },
    setup: () => ({ columns }),
    template: '<DataTable caption="Clients" row-key="id" :columns="columns" :rows="[]"><template #empty><EmptyState icon="lucide:app-window" title="No clients yet" body="Add the first app that signs people in through this issuer." /></template></DataTable>',
  }),
};
