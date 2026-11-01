import type { Meta, StoryObj } from '@storybook/vue3-vite';
import SecretReveal from './SecretReveal.vue';

const meta = {
  title: 'UI/SecretReveal',
  component: SecretReveal,
  args: { value: 'othra_cs_8f2c1b', warning: 'This secret is shown once. Store it now.', confirmLabel: 'I stored the secret', envName: 'OTHRA_CLIENT_SECRET' },
} satisfies Meta<typeof SecretReveal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Failed: Story = { args: { error: { message: 'Couldn\'t generate the secret', reference: 'Ref: req-1' } } };
