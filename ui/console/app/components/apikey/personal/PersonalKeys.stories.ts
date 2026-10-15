import type { Meta, StoryObj } from '@storybook/vue3-vite';
import { mcpSnippet } from './mcpSnippet';
import PersonalKeys from './PersonalKeys.vue';

const scopes = [
  { name: 'webhook:read', label: 'Read webhooks' },
  { name: 'webhook:write', label: 'Manage webhooks' },
  { name: 'member:read', label: 'Read members' },
];
const projects = [{ id: 'x6MH9WPWjktMwCBKHy4pF', name: 'Default' }, { id: 'KdrF3t3FHtppBPmMgj4sy', name: 'Support' }];

const meta = {
  title: 'API keys/PersonalKeys',
  component: PersonalKeys,
  args: { keys: [], scopes, projects },
} satisfies Meta<typeof PersonalKeys>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { PersonalKeys },
  setup: () => ({ args }),
  template: '<div class="max-w-3xl p-8"><PersonalKeys v-bind="args" /></div>',
});

export const Empty: Story = { render };

export const List: Story = {
  args: {
    keys: [
      { rowKey: 'key-1', name: 'laptop', scopes: ['webhook:read', 'webhook:write'], secretHint: 'a1b2' },
      { rowKey: 'key-2', name: 'old ci', scopes: ['webhook:read'], secretHint: 'c3d4', revokedAt: new Date('2026-10-01') },
    ],
  },
  render,
};

export const JustCreated: Story = {
  args: {
    keys: [{ rowKey: 'key-1', name: 'laptop', scopes: ['webhook:read'], secretHint: 'uAuE' }],
    created: {
      plaintext: 'othra_Q0QeIWMvn5Tf84kAq2gaFarjbHX2PQA__J-NsdUqAuE',
      snippet: mcpSnippet('othra', 'https://othra.example.com/mcp', 'othra_Q0QeIWMvn5Tf84kAq2gaFarjbHX2PQA__J-NsdUqAuE'),
    },
  },
  render,
};

export const SignedOut: Story = {
  args: { signedIn: false },
  render,
};
