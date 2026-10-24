import type { Meta, StoryObj } from '@storybook/vue3-vite';
import CimdCard from './CimdCard.vue';

const apps = [{ id: 'a', name: 'Claude Code', url: 'https://claude.ai/oauth/claude-code-client-metadata', firstSeen: 'Oct 23, 2026', fetched: 'Oct 23, 2026, 2:10 PM' }];

const meta = {
  title: 'Resource servers/CimdCard',
  component: CimdCard,
  args: { canEdit: true, accept: true, hosts: 'claude.ai', allScopes: true, picked: [], catalog: ['messages:read', 'messages:write'], apps },
} satisfies Meta<typeof CimdCard>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { CimdCard },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><CimdCard v-bind="args" v-model:accept="args.accept" v-model:hosts="args.hosts" v-model:all-scopes="args.allScopes" v-model:picked="args.picked" /></div>',
});

export const On: Story = { render };
export const OnlySomeScopes: Story = { args: { allScopes: false, picked: ['messages:read'] }, render };
export const AnyHost: Story = { args: { hosts: '' }, render };
export const Off: Story = { args: { accept: false, apps: [] }, render };
export const LockedUntilVerified: Story = { args: { accept: false, locked: true, apps: [] }, render };
export const BadHost: Story = { args: { hosts: '10.0.0.1', hostsError: '10.0.0.1 isn\'t a valid host name. Use a lower-case name such as claude.ai.' }, render };
export const Member: Story = { args: { canEdit: false }, render };
