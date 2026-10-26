import type { Meta, StoryObj } from '@storybook/vue3-vite';
import GoogleUpstreamCard from './GoogleUpstreamCard.vue';

const google = { clientId: '123456789012-abc.apps.googleusercontent.com', secretSet: true, enabled: true };

const meta = {
  title: 'Projects/GoogleUpstreamCard',
  component: GoogleUpstreamCard,
  args: { canEdit: true, google, clientId: google.clientId, secret: '', redirectUri: 'https://id.example.com/p/acme/federation/google/callback' },
} satisfies Meta<typeof GoogleUpstreamCard>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { GoogleUpstreamCard },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><GoogleUpstreamCard v-bind="args" v-model:client-id="args.clientId" v-model:secret="args.secret" v-model:replacing-secret="args.replacingSecret" /></div>',
});

export const SetUp: Story = { render };
export const NotSetUp: Story = { args: { google: null, clientId: '' }, render };
export const SettingUp: Story = { args: { google: null, settingUp: true, clientId: '' }, render };
export const ReplacingSecret: Story = { args: { replacingSecret: true }, render };
export const Invalid: Story = { args: { google: null, settingUp: true, clientId: '', fieldErrors: { client_id: 'Enter a value.', client_secret: 'Enter a value.' } }, render };
export const OnlyMethod: Story = { args: { locked: true }, render };
export const LinkedByEndUsers: Story = { args: { google: { ...google, enabled: false }, removeLocked: true }, render };
export const Member: Story = { args: { canEdit: false }, render };
export const Phone: Story = { render: args => ({ components: { GoogleUpstreamCard }, setup: () => ({ args }), template: '<div class="w-[358px] p-4"><GoogleUpstreamCard v-bind="args" v-model:client-id="args.clientId" v-model:secret="args.secret" /></div>' }) };
