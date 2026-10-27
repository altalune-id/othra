import type { Meta, StoryObj } from '@storybook/vue3-vite';
import VerificationStatus from './VerificationStatus.vue';

const meta = {
  title: 'Resource servers/VerificationStatus',
  component: VerificationStatus,
  args: { name: 'verified', label: 'Verified' },
} satisfies Meta<typeof VerificationStatus>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Verified: Story = {};
export const Pending: Story = { args: { name: 'pending', label: 'Pending' } };
export const Failed: Story = { args: { name: 'failed', label: 'Failed' } };
export const Trusted: Story = { args: { name: 'trusted', label: 'Trusted' } };
