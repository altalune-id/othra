import type { Meta, StoryObj } from '@storybook/vue3-vite';
import PageHeader from './PageHeader.vue';

const meta = {
  title: 'UI/PageHeader',
  component: PageHeader,
  args: { title: 'Clients', description: 'Apps that sign people in through this issuer', crumbs: [{ label: 'Acme', to: '/o/acme' }, { label: 'Clients' }] },
} satisfies Meta<typeof PageHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Issuer: Story = { args: { title: 'https://id.acme.test/p/apps', mono: true, crumbs: [] } };
export const Preview: Story = { args: { preview: true } };
