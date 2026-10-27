import type { Meta, StoryObj } from '@storybook/vue3-vite';
import PersonalKeyCreateForm from './PersonalKeyCreateForm.vue';

const orgOptions = [{ value: 'V3StGXR8Z5jdHj6BmyT2a', label: 'Acme' }, { value: 'YeDxg2kMYznPJJG3wVmjj', label: 'Globex' }];
const expiryOptions = [{ days: 7, label: '7 days' }, { days: 30, label: '30 days' }, { days: 90, label: '90 days' }, { days: 365, label: '1 year' }];
const projects = [{ id: 'x6MH9WPWjktMwCBKHy4pF', name: 'Website' }, { id: 'KdrF3t3FHtppBPmMgj4sy', name: 'App' }];
const scopeOptions = [
  { name: 'project:read', label: 'Read projects' },
  { name: 'webhook:read', label: 'Read webhooks' },
  { name: 'webhook:write', label: 'Manage webhooks' },
];

const meta = {
  title: 'API keys/PersonalKeyCreateForm',
  component: PersonalKeyCreateForm,
  args: { orgOptions, expiryOptions, projects: [], scopeOptions: [] },
} satisfies Meta<typeof PersonalKeyCreateForm>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { PersonalKeyCreateForm },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><PersonalKeyCreateForm v-bind="args" /></div>',
});

export const NoOrgPicked: Story = { render };

export const OrgPicked: Story = {
  args: { orgId: 'V3StGXR8Z5jdHj6BmyT2a', name: 'Laptop', projects, scopeOptions, scopes: ['webhook:read'], canSubmit: true },
  render,
};

export const OneOrg: Story = {
  args: { oneOrg: true, orgName: 'Acme', orgId: 'V3StGXR8Z5jdHj6BmyT2a', projects, scopeOptions, allProjects: false, projectIds: ['x6MH9WPWjktMwCBKHy4pF'] },
  render,
};

export const Failed: Story = {
  args: {
    orgId: 'YeDxg2kMYznPJJG3wVmjj',
    name: 'CI',
    scopeOptions,
    scopes: ['webhook:write'],
    canSubmit: true,
    error: { message: 'Missing permission: Manage webhooks', reference: 'Reference: req-1' },
  },
  render,
};
