import type { Meta, StoryObj } from '@storybook/vue3-vite';
import VerificationCard from './VerificationCard.vue';

const snippet = JSON.stringify({ resource: 'https://shop.example.com/mcp', authorization_servers: ['https://auth.example.com/p/acme'] }, null, 2);

const meta = {
  title: 'Resource servers/VerificationCard',
  component: VerificationCard,
  args: {
    status: 'pending',
    statusLabel: 'Pending',
    body: 'Not verified yet. Apps can\'t get tokens for it until it is.',
    metadataUrl: 'https://shop.example.com/.well-known/oauth-protected-resource/mcp',
    snippet,
    canEdit: true,
  },
} satisfies Meta<typeof VerificationCard>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { VerificationCard },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><VerificationCard v-bind="args" /></div>',
});

export const Pending: Story = { render };
export const PendingSelfHosted: Story = { args: { trustAvailable: true }, render };
export const FailedCheck: Story = {
  args: {
    failure: 'Last check, Oct 20, 2026, 1:58 PM: resource is https://shop.example.com/mcp/, not https://shop.example.com/mcp.',
  },
  render,
};
export const Verified: Story = {
  args: { status: 'verified', statusLabel: 'Verified', body: 'Verified on Oct 18, 2026. Othra checks again every day.', lastCheck: 'Oct 20, 2026, 1:58 PM', verifiedAt: 'Oct 18, 2026, 9:12 AM', snippet: '' },
  render,
};
export const VerifiedRecheckFailing: Story = {
  args: { status: 'verified', statusLabel: 'Verified', body: 'Verified on Oct 18, 2026. Othra checks again every day.', recheck: 'The last 1 of 3 checks failed: No answer within 5 seconds. After 3, apps can\'t get tokens for it.', snippet: '' },
  render,
};
export const Trusted: Story = {
  args: { status: 'trusted', statusLabel: 'Trusted', body: 'Trusted without verification. Othra never checks it.', trustAvailable: true },
  render,
};
export const HttpLocked: Story = {
  args: { status: 'trusted', statusLabel: 'Trusted', body: 'Trusted without verification. Othra never checks it.', trustAvailable: true, http: true, metadataUrl: '' },
  render,
};
export const Member: Story = { args: { canEdit: false, status: 'failed', statusLabel: 'Failed', body: 'Apps can\'t get tokens for it. Fix the metadata, then verify again.' }, render };
