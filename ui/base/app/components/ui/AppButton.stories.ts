import type { Meta, StoryObj } from '@storybook/vue3-vite';
import AppButton from './AppButton.vue';

const meta = {
  title: 'UI/AppButton',
  component: AppButton,
  args: { variant: 'primary', size: 'sm' },
  argTypes: {
    variant: { control: 'select', options: ['primary', 'neutral', 'ghost', 'outline', 'warning', 'error', 'plain'] },
    size: { control: 'inline-radio', options: ['xs', 'sm', 'md', 'lg'] },
    type: { control: 'select', options: ['button', 'submit', 'reset'] },
  },
} satisfies Meta<typeof AppButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Playground: Story = {
  render: args => ({
    components: { AppButton },
    setup: () => ({ args }),
    template: '<AppButton v-bind="args">New client</AppButton>',
  }),
};

export const ConsoleSet: Story = {
  name: 'Console set (artifact)',
  render: () => ({
    components: { AppButton },
    template: `
      <div class="flex flex-wrap items-center gap-2">
        <AppButton size="sm"><Icon name="lucide:plus" size="16" />New client</AppButton>
        <AppButton variant="plain" size="sm"><Icon name="lucide:copy" size="16" />Copy issuer</AppButton>
        <AppButton variant="plain" size="sm"><Icon name="lucide:braces" size="16" />View as JSON</AppButton>
        <AppButton variant="ghost" size="sm" class="text-base-content/70"><Icon name="lucide:search" size="16" />Search<kbd class="kbd kbd-xs">⌘K</kbd></AppButton>
        <AppButton variant="ghost" size="xs" square aria-label="Actions for Storefront web"><Icon name="lucide:ellipsis" size="16" /></AppButton>
        <AppButton variant="ghost" size="sm" square aria-label="Toggle navigation"><Icon name="lucide:menu" size="18" /></AppButton>
      </div>`,
  }),
};

export const HostedSet: Story = {
  name: 'Sign-in set (artifact)',
  render: () => ({
    components: { AppButton },
    template: `
      <div class="flex w-80 flex-col gap-2">
        <AppButton class="w-full">Continue</AppButton>
        <AppButton variant="plain" class="w-full">Continue with Google</AppButton>
        <div class="flex gap-2">
          <AppButton variant="plain" class="flex-1">Cancel</AppButton>
          <AppButton class="flex-1">Allow access</AppButton>
        </div>
      </div>`,
  }),
};

export const States: Story = {
  render: () => ({
    components: { AppButton },
    template: `
      <div class="flex flex-wrap items-center gap-2">
        <AppButton size="sm" busy>Saving</AppButton>
        <AppButton size="sm" disabled>Continue to quickstart</AppButton>
        <AppButton variant="error" size="sm" soft>Delete client</AppButton>
      </div>`,
  }),
};
