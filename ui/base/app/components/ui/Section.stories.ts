import type { Meta, StoryObj } from '@storybook/vue3-vite';
import Section from './Section.vue';

const meta = {
  title: 'UI/Section',
  component: Section,
  args: { title: 'Issuer status', description: 'What your apps see.' },
} satisfies Meta<typeof Section>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Danger: Story = { args: { title: 'Danger zone', tone: 'danger' } };
