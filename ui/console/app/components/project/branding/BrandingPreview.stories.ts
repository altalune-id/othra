import type { Meta, StoryObj } from '@storybook/vue3-vite';
import BrandingPreview from './BrandingPreview.vue';

const model = {
  name: 'Acme App',
  logoUrl: '',
  primary: '#4f46e5',
  primaryContent: '#ffffff',
  termsUrl: 'https://acme.example.com/terms',
  privacyUrl: 'https://acme.example.com/privacy',
  supportEmail: 'help@acme.example.com',
};

const meta = {
  title: 'Projects/BrandingPreview',
  component: BrandingPreview,
  args: { model, theme: 'othra-light' },
} satisfies Meta<typeof BrandingPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

const render: Story['render'] = args => ({
  components: { BrandingPreview },
  setup: () => ({ args }),
  template: '<div class="max-w-sm p-8"><BrandingPreview v-bind="args" /></div>',
});

export const Light: Story = { render };
export const Dark: Story = { args: { theme: 'othra-dark' }, render };
export const NoLogoNoLinks: Story = { args: { model: { ...model, termsUrl: '', privacyUrl: '', supportEmail: '' } }, render };
export const DefaultColour: Story = { args: { model: { ...model, primary: '', primaryContent: '' } }, render };
export const LowContrast: Story = { args: { model: { ...model, primary: '#777777', primaryContent: '#ffffff' } }, render };
export const WithLogo: Story = {
  args: {
    model: {
      ...model,
      logoUrl: 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1" fill="%234f46e5"/></svg>',
    },
  },
  render,
};
