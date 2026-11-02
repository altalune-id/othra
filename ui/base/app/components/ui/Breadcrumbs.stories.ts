import type { Meta, StoryObj } from '@storybook/vue3-vite';
import Breadcrumbs from './Breadcrumbs.vue';

const meta = {
  title: 'UI/Breadcrumbs',
  component: Breadcrumbs,
  args: { crumbs: [{ label: 'Acme', to: '#org' }, { label: 'acme-prod', to: '#project', mono: true }, { label: 'Overview' }] },
} satisfies Meta<typeof Breadcrumbs>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Project: Story = { name: 'Org › project › page (artifact)' };

export const Clients: Story = {
  args: { crumbs: [{ label: 'Acme', to: '#org' }, { label: 'acme-prod', to: '#project', mono: true }, { label: 'Clients' }] },
};
