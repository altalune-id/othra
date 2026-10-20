import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ClientForm from './ClientForm.vue';

const kinds = [
  { value: 'web' as const, icon: 'lucide:globe', label: 'Web app', help: 'Runs on a server and keeps a secret.', redirectHelp: 'For example https://app.example.com/oauth/callback. For local dev, http://127.0.0.1:5100/… with the exact port.', placeholder: 'https://app.example.com/oauth/callback', postLogout: true },
  { value: 'cli' as const, icon: 'lucide:terminal', label: 'CLI or desktop app', help: 'Runs on the end user\'s computer. Has no secret.', redirectHelp: 'For example http://127.0.0.1/callback. Leave out the port: any port works.', placeholder: 'http://127.0.0.1/callback', postLogout: false },
];

const meta = {
  title: 'Clients/ClientForm',
  component: ClientForm,
  args: {
    kinds,
    fields: { kind: 'web', name: '', redirectUris: [''], postLogoutUris: [], refreshTokens: false, consent: 'require' },
  },
} satisfies Meta<typeof ClientForm>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { ClientForm },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><div class="card border border-base-300 bg-base-100"><div class="card-body"><ClientForm v-bind="args" v-model:fields="args.fields" @kind="k => (args.fields = { ...args.fields, kind: k })" /></div></div></div>',
});

export const NewWebApp: Story = { render };
export const NewCliApp: Story = {
  args: { fields: { kind: 'cli', name: 'Storefront CLI', redirectUris: ['http://127.0.0.1/callback'], postLogoutUris: [], refreshTokens: true, consent: 'require' } },
  render,
};
export const Errors: Story = {
  args: {
    fields: { kind: 'web', name: '', redirectUris: ['https://shop.example.com/cb#x', 'http://localhost:5173/cb'], postLogoutUris: [], refreshTokens: true, consent: 'implicit' },
    nameError: 'This field is required.',
    uriErrors: { redirect: ['It can\'t contain #.', ''], postLogout: [] },
    warnings: { redirect: [false, true], postLogout: [] },
    error: { message: 'Failed to save the client', reference: 'Ref: req-1' },
  },
  render,
};
export const Edit: Story = {
  args: {
    mode: 'edit',
    fields: { kind: 'web', name: 'Storefront web', redirectUris: ['https://shop.example.com/oauth/callback'], postLogoutUris: ['https://shop.example.com/'], refreshTokens: true, consent: 'require' },
  },
  render,
};
export const Busy: Story = { args: { busy: true }, render };
