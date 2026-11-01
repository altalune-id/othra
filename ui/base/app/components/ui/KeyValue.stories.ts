import type { Meta, StoryObj } from '@storybook/vue3-vite';
import KeyValue from './KeyValue.vue';

const meta = {
  title: 'UI/KeyValue',
  component: KeyValue,
  args: { items: [{ label: 'Client ID', value: 'dPm7fmQRJKPVw8PxrgdrN', mono: true, copy: true }, { label: 'Type', value: 'Web' }, { label: 'Logo' }] },
} satisfies Meta<typeof KeyValue>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
