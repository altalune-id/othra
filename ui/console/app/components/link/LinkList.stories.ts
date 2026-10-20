import type { Meta, StoryObj } from '@storybook/vue3-vite';
import LinkList from './LinkList.vue';

const rows = [
  { id: 'a', name: 'Storefront API', detail: 'https://api.shop.example.com', scopes: ['orders:read', 'orders:write'], status: { name: 'verified' as const, label: 'Verified' } },
  { id: 'b', name: 'Storefront MCP', detail: 'https://shop.example.com/mcp', scopes: ['mcp:tools'], status: { name: 'pending' as const, label: 'Pending' } },
];

const meta = {
  title: 'Links/LinkList',
  component: LinkList,
  args: { rows, canEdit: true },
} satisfies Meta<typeof LinkList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { LinkList },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><LinkList v-bind="args" /></div>',
});

export const OnAClient: Story = { render };
export const OnAResourceServer: Story = {
  args: {
    rows: [{ id: 'c', name: 'Storefront web', detail: 'Web app · dPm7fmQRJKPVw8PxrgdrN', scopes: ['orders:read'] }],
    labels: { title: 'Clients', link: 'Link client', empty: 'Not linked yet. Link one so it can ask for its scopes.', emptyReadOnly: 'Not linked yet.', edit: 'Edit', unlink: 'Unlink' },
  },
  render,
};
export const Empty: Story = { args: { rows: [] }, render };
export const Member: Story = { args: { canEdit: false }, render };
export const MemberEmpty: Story = { args: { rows: [], canEdit: false }, render };
export const Loading: Story = { args: { rows: [], loading: true }, render };
