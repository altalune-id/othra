import type { Meta, StoryObj } from '@storybook/vue3-vite';

const meta = {
  title: 'Foundations',
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

const scale = [
  ['text-xl', 'Page title · Acme'],
  ['text-lg', 'Section title · Issuer status'],
  ['text-base', 'Body · People sign in to your apps through this issuer.'],
  ['text-sm', 'Small · Apps that sign people in through this issuer'],
  ['text-ui', 'UI · Clients, Resource servers, End users'],
  ['text-xs', 'Caption · Checked just now'],
];

const colors = [
  ['primary', 'bg-primary text-primary-content'],
  ['secondary', 'bg-secondary text-secondary-content'],
  ['accent', 'bg-accent text-accent-content'],
  ['neutral', 'bg-neutral text-neutral-content'],
  ['info', 'bg-info text-info-content'],
  ['success', 'bg-success text-success-content'],
  ['warning', 'bg-warning text-warning-content'],
  ['error', 'bg-error text-error-content'],
];
const bases = [['base-100', 'bg-base-100'], ['base-200', 'bg-base-200'], ['base-300', 'bg-base-300']];

export const Type: Story = {
  render: () => ({
    setup: () => ({ scale }),
    template: `
      <div class="space-y-3 bg-base-100 p-6 text-base-content">
        <div v-for="[cls, text] in scale" :key="cls" class="flex items-baseline gap-4">
          <code class="w-20 shrink-0 font-mono text-xs text-base-content/70">{{ cls }}</code>
          <span :class="cls">{{ text }}</span>
        </div>
        <div class="flex items-baseline gap-4">
          <code class="w-20 shrink-0 font-mono text-xs text-base-content/70">font-mono</code>
          <span class="font-mono text-ui">https://othra.example/p/acme-prod · acme-prod</span>
        </div>
        <div class="flex items-baseline gap-4">
          <code class="w-20 shrink-0 font-mono text-xs text-base-content/70">weights</code>
          <span class="text-ui">Regular 400 · <span class="font-medium">Medium 500</span> · <span class="font-semibold">Semibold 600</span></span>
        </div>
      </div>`,
  }),
};

export const Colors: Story = {
  render: () => ({
    setup: () => ({ colors, bases }),
    template: `
      <div class="space-y-4 bg-base-100 p-6 text-ui text-base-content">
        <div class="flex flex-wrap gap-2">
          <div v-for="[name, cls] in bases" :key="name" class="flex h-16 w-32 items-end rounded-box border border-base-300 p-2" :class="cls">{{ name }}</div>
        </div>
        <div class="flex flex-wrap gap-2">
          <div v-for="[name, cls] in colors" :key="name" class="flex h-16 w-32 items-end rounded-box p-2" :class="cls">{{ name }}</div>
        </div>
        <p class="text-base-content/70">Muted text is base-content/70.</p>
      </div>`,
  }),
};

export const RadiiAndDensity: Story = {
  render: () => ({
    template: `
      <div class="space-y-4 bg-base-100 p-6 text-ui text-base-content">
        <div class="flex flex-wrap gap-3">
          <div class="rounded-box border border-base-300 p-3">rounded-box</div>
          <div class="rounded-field border border-base-300 p-3">rounded-field</div>
          <div class="rounded-selector border border-base-300 p-3">rounded-selector</div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-sm text-ui btn-primary">New client</button>
          <button type="button" class="btn btn-sm text-ui">Copy issuer</button>
          <button type="button" class="btn btn-sm text-ui btn-ghost">Cancel</button>
          <input class="input text-ui w-56" placeholder="Search clients" aria-label="Search clients">
        </div>
        <div class="navbar min-h-12 border border-base-300 bg-base-100 px-4">Top bar · min-h-12</div>
      </div>`,
  }),
};
