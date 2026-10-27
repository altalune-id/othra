import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ClientList from './ClientList.vue';

const rows = [
  { id: 'Lq3vT8nWx2KpZr6YbM9cD', name: 'Othra console', kind: 'Web app', created: 'Oct 20, 2026', to: '#', system: true },
  { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Storefront web', kind: 'Web app', created: 'Oct 20, 2026', to: '#' },
  { id: 'H9MspUj32ux8pfYKYsu2a', name: 'Storefront CLI', kind: 'CLI or desktop app', created: 'Oct 20, 2026', to: '#' },
];

const meta = {
  title: 'Clients/ClientList',
  component: ClientList,
  args: { rows },
} satisfies Meta<typeof ClientList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ClientList },
  setup: () => ({ args }),
  template: '<div class="max-w-4xl p-8"><div class="card border border-base-300 bg-base-100"><div class="card-body"><ClientList v-bind="args" /></div></div></div>',
});

export const Default: Story = { render };
export const Loading: Story = { args: { rows: [], loading: true }, render };
export const Mobile: Story = {
  render: args => ({
    components: { ClientList },
    setup: () => ({ args }),
    template: '<div class="w-[358px] p-4"><ClientList v-bind="args" /></div>',
  }),
};
