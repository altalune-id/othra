import type { Meta, StoryObj } from '@storybook/vue3-vite';
import IssuerHandleField from './IssuerHandleField.vue';

const meta = {
  title: 'Projects/IssuerHandleField',
  component: IssuerHandleField,
  args: { modelValue: 'acme-web', status: 'idle', prefix: 'https://auth.example.com/p/' },
} satisfies Meta<typeof IssuerHandleField>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { IssuerHandleField },
  setup: () => ({ args }),
  template: '<div class="max-w-xl p-8"><IssuerHandleField v-bind="args" /></div>',
});

export const Idle: Story = { render };
export const Checking: Story = { args: { modelValue: 'acme-shop', status: 'checking' }, render };
export const Available: Story = { args: { modelValue: 'acme-shop', status: 'available' }, render };
export const Taken: Story = { args: { modelValue: 'acme', status: 'taken' }, render };
export const Reserved: Story = { args: { modelValue: 'console', status: 'reserved' }, render };
export const Invalid: Story = { args: { modelValue: 'Acme_Web', status: 'invalid' }, render };
export const CheckFailed: Story = { args: { modelValue: 'acme-shop', status: 'error' }, render };
export const Locked: Story = { args: { status: 'locked' }, render };
export const InCreateModal: Story = { args: { modelValue: 'acme-support-bot', status: 'available', prefix: '' }, render };
export const ServerRefused: Story = { args: { modelValue: 'acme-shop', status: 'idle', fieldError: 'The issuer handle acme-shop is already taken.' }, render };
