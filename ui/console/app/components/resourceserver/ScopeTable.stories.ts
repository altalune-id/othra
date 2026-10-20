import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ScopeTable from './ScopeTable.vue';

const meta = {
  title: 'Resource servers/ScopeTable',
  component: ScopeTable,
  args: { rows: ['mcp:tools', 'orders:read'] },
} satisfies Meta<typeof ScopeTable>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ScopeTable },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><ScopeTable v-bind="args" v-model:rows="args.rows" @add="args.rows = [...args.rows, \'\']" /></div>',
});

export const Editable: Story = { render };
export const Empty: Story = { args: { rows: [''] }, render };
export const WithError: Story = { args: { rows: ['mcp:tools', 'openid'], errors: ['', 'openid, profile, email and offline_access are built in.'] }, render };
export const ReadOnly: Story = { args: { editable: false }, render };
