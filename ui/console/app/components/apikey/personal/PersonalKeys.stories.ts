import type { Meta, StoryObj } from '@storybook/vue3-vite';
import type { PersonalKeyView } from './PersonalKeys.vue';
import { mcpSnippet } from './mcpSnippet';
import PersonalKeys from './PersonalKeys.vue';

const acme = 'V3StGXR8Z5jdHj6BmyT2a';
const globex = 'YeDxg2kMYznPJJG3wVmjj';

const keys: PersonalKeyView[] = [
  { rowKey: 'key-1', orgId: acme, orgName: 'Acme', name: 'Laptop', secretHint: 'a1b2', scopes: ['Read webhooks', 'Manage webhooks'], lastUsed: '2 hours ago', expires: 'Jan 14, 2027', expiry: { kind: 'ok' }, revoked: false },
  { rowKey: 'key-2', orgId: globex, orgName: 'Globex', name: 'CI', secretHint: 'c3d4', scopes: ['Read projects'], lastUsed: '', expires: 'Oct 6, 2026', expiry: { kind: 'soon', days: 5 }, revoked: false },
  { rowKey: 'key-3', orgId: globex, orgName: 'Globex', name: 'Old script', secretHint: 'e5f6', scopes: ['Read projects'], lastUsed: '2 months ago', expires: 'Sep 1, 2026', expiry: { kind: 'expired' }, revoked: false },
  { rowKey: 'key-4', orgId: globex, orgName: 'Globex', name: 'Retired', secretHint: 'g7h8', scopes: ['Read webhooks'], lastUsed: '30 days ago', expires: '', expiry: { kind: 'ok' }, revoked: true },
];

const meta = {
  title: 'API keys/PersonalKeys',
  component: PersonalKeys,
  args: { keys },
} satisfies Meta<typeof PersonalKeys>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { PersonalKeys },
  setup: () => ({ args }),
  template: '<div class="max-w-5xl p-8"><PersonalKeys v-bind="args" /></div>',
});

export const SeveralOrgs: Story = { render };

export const OneOrg: Story = {
  args: { keys: keys.filter(k => k.orgId === globex), oneOrg: true },
  render,
};

export const Empty: Story = {
  args: { keys: [] },
  render,
};

export const PartialFailure: Story = {
  args: { keys: keys.filter(k => k.orgId === acme), failed: [{ id: globex, name: 'Globex' }] },
  render,
};

export const JustCreated: Story = {
  args: {
    created: {
      plaintext: 'othra_Q0QeIWMvn5Tf84kAq2gaFarjbHX2PQA__J-NsdUqAuE',
      snippet: mcpSnippet('othra', 'https://othra.example.com/mcp', 'othra_Q0QeIWMvn5Tf84kAq2gaFarjbHX2PQA__J-NsdUqAuE'),
    },
  },
  render,
};

export const SignedOut: Story = {
  args: { keys: [], signedIn: false },
  render,
};
