import type { Meta, StoryObj } from '@storybook/vue3-vite';
import FormField from './FormField.vue';
import SearchInput from './SearchInput.vue';

const meta = {
  title: 'UI/Inputs',
  component: FormField,
  args: { label: 'Client name', hint: 'People see it on the consent screen', error: '' },
} satisfies Meta<typeof FormField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: args => ({
    components: { FormField },
    setup: () => ({ args }),
    template: `<FormField v-bind="args" v-slot="{ id, describedBy, invalid }"><input :id="id" class="input input-sm text-ui w-full" value="Acme Orders" :aria-describedby="describedBy" :aria-invalid="invalid"></FormField>`,
  }),
};

export const Fields: Story = {
  name: 'Fields (artifact)',
  render: () => ({
    components: { FormField },
    template: `
      <div class="flex max-w-xl flex-col gap-3">
        <FormField label="Client name" v-slot="{ id, describedBy }"><input :id="id" class="input input-sm text-ui w-full" value="Acme Orders" :aria-describedby="describedBy"></FormField>
        <FormField label="Redirect URI" v-slot="{ id, describedBy }"><input :id="id" type="url" class="input input-sm text-ui w-full font-mono" value="https://orders.acme.example/callback" :aria-describedby="describedBy"></FormField>
        <FormField label="Signing algorithm" hint="Apps verify tokens with the published keys" v-slot="{ id, describedBy }"><select :id="id" class="select select-sm text-ui w-full font-mono" :aria-describedby="describedBy"><option>RS256</option><option>ES256</option></select></FormField>
        <FormField label="Name" error="Enter a name that people will recognise on the consent screen" v-slot="{ id, describedBy, invalid }"><input :id="id" class="input input-sm text-ui w-full" :aria-describedby="describedBy" :aria-invalid="invalid"></FormField>
        <FormField label="Budget per run" v-slot="{ id, describedBy }"><label class="input input-sm text-ui w-full">$<input :id="id" type="number" step="0.01" value="0.25" :aria-describedby="describedBy"></label></FormField>
        <FormField label="Temperature" hint="Lower is more consistent" v-slot="{ id, describedBy }"><input :id="id" type="range" min="0" max="10" value="2" class="range range-xs w-full" :aria-describedby="describedBy"></FormField>
      </div>`,
  }),
};

export const Search: Story = {
  name: 'Search with icon (artifact)',
  render: () => ({
    components: { SearchInput },
    template: `<SearchInput label="Filter agents" />`,
  }),
};

export const Choices: Story = {
  name: 'Checkbox and toggle (artifact)',
  render: () => ({
    template: `
      <div class="flex flex-col gap-2">
        <label class="label text-ui text-base-content"><input type="checkbox" class="checkbox checkbox-sm rounded-sm" checked disabled> Require PKCE</label>
        <label class="label text-ui text-base-content"><input type="checkbox" class="checkbox checkbox-sm rounded-sm" checked> Issue refresh tokens</label>
        <label class="label text-ui text-base-content"><input type="checkbox" class="checkbox checkbox-sm rounded-sm"> Skip consent for this first-party app</label>
        <label class="label text-ui text-base-content"><input type="checkbox" class="toggle toggle-sm" checked> Hide reads</label>
      </div>`,
  }),
};

export const RadioCards: Story = {
  name: 'Radio cards (artifact)',
  render: () => ({
    template: `
      <div class="grid gap-2 sm:grid-cols-2">
        <label class="flex cursor-pointer items-start gap-3 rounded-box border border-base-300 p-3 has-checked:border-primary has-checked:bg-base-200">
          <input type="radio" class="radio radio-sm mt-0.5" name="client-type" value="web" checked>
          <span class="min-w-0"><span class="flex items-center gap-2 text-sm font-medium"><Icon name="lucide:app-window" size="16" class="shrink-0" aria-hidden="true" />Web app</span><span class="block text-ui text-base-content/70">Runs on your server</span><span class="block text-xs text-base-content/70">Recommended: Authorization code with PKCE and a client secret</span></span>
        </label>
        <label class="flex cursor-pointer items-start gap-3 rounded-box border border-base-300 p-3 has-checked:border-primary has-checked:bg-base-200">
          <input type="radio" class="radio radio-sm mt-0.5" name="client-type" value="spa">
          <span class="min-w-0"><span class="flex items-center gap-2 text-sm font-medium"><Icon name="lucide:panels-top-left" size="16" class="shrink-0" aria-hidden="true" />Single-page app</span><span class="block text-ui text-base-content/70">Runs in the browser</span><span class="block text-xs text-base-content/70">Recommended: Authorization code with PKCE, no secret</span></span>
        </label>
      </div>`,
  }),
};
