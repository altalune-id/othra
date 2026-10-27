import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ClientSecretDialog from './ClientSecretDialog.vue';

const meta = {
  title: 'Clients/ClientSecretDialog',
  component: ClientSecretDialog,
  args: { open: true, ownerId: 'dPm7fmQRJKPVw8PxrgdrN', secret: 'othra_cs_3kQvT9xR2mWb7nLp4aRvE5sQ8uC6dHfYz0JgKeNtUoA' },
} satisfies Meta<typeof ClientSecretDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
