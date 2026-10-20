import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ClientSecrets from './ClientSecrets.vue';

const current = { id: 'a', hint: 'Xk3Q', old: false, created: 'Oct 20, 2026, 2:02 PM', expires: '' };
const old = { id: 'b', hint: 'p7Lm', old: true, created: 'Oct 1, 2026, 9:00 AM', expires: 'Oct 21, 2026, 2:02 PM' };

const meta = {
  title: 'Clients/ClientSecrets',
  component: ClientSecrets,
  args: { rows: [current], canEdit: true },
} satisfies Meta<typeof ClientSecrets>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ClientSecrets },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><ClientSecrets v-bind="args" /></div>',
});

export const OneSecret: Story = { render };
export const DuringGrace: Story = { args: { rows: [current, old] }, render };
export const NoSecretYet: Story = { args: { rows: [] }, render };
export const Member: Story = { args: { rows: [current, old], canEdit: false }, render };
export const Refused: Story = { args: { error: { message: 'Only a signed-in person can create or rotate a secret.' } }, render };
