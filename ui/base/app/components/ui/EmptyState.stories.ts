import type { Meta, StoryObj } from '@storybook/vue3-vite';
import EmptyState from './EmptyState.vue';

const meta = {
  title: 'UI/EmptyState',
  component: EmptyState,
  args: { icon: 'lucide:app-window', title: 'No clients yet', body: 'A client is an app that signs people in through this issuer.' },
} satisfies Meta<typeof EmptyState>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
