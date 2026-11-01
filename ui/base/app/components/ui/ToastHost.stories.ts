import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ToastHost from './ToastHost.vue';

const meta = {
  title: 'UI/ToastHost',
  component: ToastHost,
  args: {},
} satisfies Meta<typeof ToastHost>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {};
