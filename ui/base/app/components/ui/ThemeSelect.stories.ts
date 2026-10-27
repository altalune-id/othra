import type { Meta, StoryObj } from '@storybook/vue3-vite';
import ThemeSelect from './ThemeSelect.vue';

const meta = {
  title: 'UI/ThemeSelect',
  component: ThemeSelect,
  args: {
    modelValue: 'othra',
    label: 'Theme',
    open: true,
    families: [
      { id: 'othra', label: 'Othra', theme: 'othra-light' },
      { id: 'bumblebee', label: 'Bumblebee', theme: 'bumblebee-light' },
      { id: 'retro', label: 'Retro', theme: 'retro-light' },
      { id: 'coffee', label: 'Coffee', theme: 'coffee-dark' },
      { id: 'night', label: 'Night', theme: 'night-dark' },
    ],
  },
} satisfies Meta<typeof ThemeSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: args => ({
    components: { ThemeSelect },
    setup: () => ({ args }),
    template: '<div class="flex justify-end p-24"><ThemeSelect v-bind="args" /></div>',
  }),
};
