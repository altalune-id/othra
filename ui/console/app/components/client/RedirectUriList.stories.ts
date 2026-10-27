import type { Meta, StoryObj } from '@storybook/vue3-vite';
import RedirectUriList from './RedirectUriList.vue';

const meta = {
  title: 'Clients/RedirectUriList',
  component: RedirectUriList,
  args: {
    label: 'Redirect URIs',
    help: 'One per line, for example https://app.example.com/oauth/callback. For local dev, http://127.0.0.1:5100/… with the exact port.',
    rows: ['https://shop.example.com/oauth/callback'],
  },
} satisfies Meta<typeof RedirectUriList>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { RedirectUriList },
  setup: () => ({ args }),
  template: '<div class="max-w-2xl p-8"><RedirectUriList v-bind="args" v-model:rows="args.rows" /></div>',
});

export const One: Story = { render };
export const Empty: Story = { args: { rows: [''], placeholder: 'https://app.example.com/oauth/callback' }, render };
export const WithErrorAndWarning: Story = {
  args: {
    rows: ['https://shop.example.com/oauth/callback', 'https://shop.example.com/cb#x', 'http://localhost:5173/oauth/callback'],
    errors: ['', 'It can\'t contain #.', ''],
    warnings: [false, false, true],
  },
  render,
};
export const Optional: Story = { args: { label: 'Sign-out return URIs', rows: [], required: false }, render };
