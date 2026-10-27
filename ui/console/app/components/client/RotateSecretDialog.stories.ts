import type { Meta, StoryObj } from '@storybook/vue3-vite';
import RotateSecretDialog from './RotateSecretDialog.vue';

const options = [
  { value: 1, label: 'Right away (stops now)' },
  { value: 2, label: '1 hour' },
  { value: 3, label: '1 day' },
  { value: 4, label: '7 days' },
];

const meta = {
  title: 'Clients/RotateSecretDialog',
  component: RotateSecretDialog,
  args: { open: true, options, nowValue: 1, grace: 3 },
} satisfies Meta<typeof RotateSecretDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const RightAway: Story = { args: { grace: 1 } };
export const EndsOldSecret: Story = { args: { oldHint: 'p7Lm' } };
export const Busy: Story = { args: { busy: true } };
export const Refused: Story = { args: { error: { message: 'Only a signed-in person can create or rotate a secret.', reference: 'Ref: req-1' } } };
