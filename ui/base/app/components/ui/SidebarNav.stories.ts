import type { Meta, StoryObj } from '@storybook/vue3-vite';
import SidebarNav from './SidebarNav.vue';

const meta = {
  title: 'UI/SidebarNav',
  component: SidebarNav,
  args: {
    label: 'acme-prod',
    groups: [
      { items: [{ key: 'overview', label: 'Overview', to: '#overview', icon: 'lucide:activity', active: true }] },
      { label: 'Applications', items: [{ key: 'clients', label: 'Clients', to: '#clients', icon: 'lucide:app-window' }, { key: 'rs', label: 'Resource servers', to: '#rs', icon: 'lucide:server' }] },
      { label: 'People', items: [{ key: 'end-users', label: 'End users', to: '#end-users', icon: 'lucide:user-round' }] },
      { label: 'Sign-in', items: [{ key: 'methods', label: 'Methods', to: '#methods', icon: 'lucide:log-in' }, { key: 'branding', label: 'Branding', to: '#branding', icon: 'lucide:palette' }, { key: 'legal', label: 'Legal', to: '#legal', icon: 'lucide:scale' }] },
      { label: 'Security', items: [{ key: 'keys', label: 'Signing keys', to: '#keys', icon: 'lucide:key-square' }, { key: 'issuer', label: 'Issuer', to: '#issuer', icon: 'lucide:badge-check' }] },
      { label: 'Developers', items: [{ key: 'webhooks', label: 'Webhooks', to: '#webhooks', icon: 'lucide:webhook' }] },
    ],
  },
} satisfies Meta<typeof SidebarNav>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: args => ({
    components: { SidebarNav },
    setup: () => ({ args }),
    template: '<div class="flex h-[40rem]"><SidebarNav v-bind="args" /></div>',
  }),
};

export const WithFooter: Story = {
  render: args => ({
    components: { SidebarNav },
    setup: () => ({ args }),
    template: '<div class="flex h-[40rem]"><SidebarNav v-bind="args"><template #footer><span class="px-2 text-xs text-base-content/70">Othra v0.1.0-dev</span></template></SidebarNav></div>',
  }),
};
