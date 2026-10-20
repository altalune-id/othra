import type { Meta, StoryObj } from '@storybook/vue3-vite';
import type { KeyRow } from './keyRows';
import SigningKeyList from './SigningKeyList.vue';

const rows: KeyRow[] = [
  { id: 'Kp1n2Xq8RtY5uVw3zA9bC', kid: '3f9a1c2e9d8b7a6f5e4d', short: '3f9a1c2e', alg: 'RS256', status: 'next', help: 'Published. Starts signing at about 14:32.', created: 'Oct 18, 2026', retires: '' },
  { id: 'Lq2o3Yr9SuZ6vWx4aB0cD', kid: '8b20d4f1aa00bb11cc22', short: '8b20d4f1', alg: 'RS256', status: 'active', help: '', created: 'Oct 10, 2026', retires: '' },
  { id: 'Mr3p4Zs0TvA7wXy5bC1dE', kid: '1c77e0aa12345678abcd', short: '1c77e0aa', alg: 'RS256', status: 'retiring', help: 'No longer signs. Still published until Oct 24, 2026, so tokens it signed keep working.', created: 'Sep 30, 2026', retires: 'Oct 24, 2026' },
  { id: 'Ns4q5At1UwB8xYz6cD2eF', kid: '9e01b3c555556666aaaa', short: '9e01b3c5', alg: 'RS256', status: 'retired', help: '', created: 'Sep 1, 2026', retires: 'Oct 17, 2026' },
];

const meta = {
  title: 'Projects/SigningKeyList',
  component: SigningKeyList,
  args: { rows },
} satisfies Meta<typeof SigningKeyList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { SigningKeyList },
  setup: () => ({ args }),
  template: '<div class="max-w-4xl p-8"><div class="card border border-base-300 bg-base-100"><div class="card-body"><SigningKeyList v-bind="args" /></div></div></div>',
});

export const AllStatuses: Story = { render };
export const OnlyActive: Story = { args: { rows: [rows[1]!] }, render };
export const Loading: Story = { args: { rows: [], loading: true }, render };
export const Mobile: Story = {
  render: args => ({
    components: { SigningKeyList },
    setup: () => ({ args }),
    template: '<div class="w-[358px] p-4"><SigningKeyList v-bind="args" /></div>',
  }),
};
