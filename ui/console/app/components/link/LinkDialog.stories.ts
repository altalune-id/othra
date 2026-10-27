import type { Meta, StoryObj } from '@storybook/vue3-vite';
import LinkDialog from './LinkDialog.vue';

const targets = [
  { value: 'a', label: 'Storefront API' },
  { value: 'b', label: 'Storefront MCP' },
];

const meta = {
  title: 'Links/LinkDialog',
  component: LinkDialog,
  args: { open: true, targets, targetId: 'a', scopeOptions: ['orders:read', 'orders:write'], picked: ['orders:read'] },
} satisfies Meta<typeof LinkDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const New: Story = {};
export const Unverified: Story = { args: { unverified: true } };
export const PickAScope: Story = { args: { picked: [], pickError: true } };
export const Edit: Story = { args: { mode: 'edit', fixedName: 'Storefront API', labels: { title: 'Edit allowed scopes', target: 'Resource server', scopes: 'Allowed scopes', scopesHelp: 'The client can ask only for these. Pick at least one.', pickScope: 'Pick at least one scope.', unverified: '', noScopes: 'Add scopes to this resource server first.', noneLeft: '', cancel: 'Cancel', confirm: 'Save' } } };
export const NoScopes: Story = { args: { scopeOptions: [] } };
export const NothingLeft: Story = { args: { targets: [] } };
export const Refused: Story = { args: { error: { message: 'orders:write isn\'t a scope of this resource server.' } } };
