import type { Meta, StoryObj } from '@storybook/vue3-vite';
import EntitySelect from './EntitySelect.vue';

const meta = {
  title: 'UI/EntitySelect',
  component: EntitySelect,
  args: {
    label: 'Switch organization',
    searchLabel: 'Search organizations',
    title: 'Last opened',
    open: true,
    items: [
      { key: 'acme', label: 'Acme', active: true },
      { key: 'north', label: 'Northwind Labs' },
    ],
  },
} satisfies Meta<typeof EntitySelect>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Organization: Story = {
  name: 'Org switcher (artifact)',
  render: args => ({
    components: { EntitySelect },
    setup: () => ({ args }),
    template: `
      <div class="h-80 w-60 bg-base-200 p-2">
        <EntitySelect v-bind="args">
          <template #trigger>
            <span class="inline-flex min-w-0 items-center gap-2"><img src="/static/altalune-logo.png" alt="" width="16" height="16" class="size-4 shrink-0 object-contain"><span class="font-semibold">Othra</span><span class="text-base-content/70">/</span><span class="truncate">Acme</span></span>
          </template>
          <template #actions>
            <li><button type="button"><Icon name="lucide:plus" size="16" class="shrink-0" />New organization</button></li>
          </template>
        </EntitySelect>
      </div>`,
  }),
};

export const Project: Story = {
  name: 'Project row (artifact)',
  args: {
    label: 'Switch project',
    searchLabel: 'Search projects',
    title: '',
    triggerClass: 'h-auto min-h-8 py-1',
    items: [
      { key: 'acme-prod', label: 'acme-prod', mono: true, badge: 'Production', active: true },
      { key: 'acme-labs', label: 'acme-labs', mono: true, badge: 'Labs' },
    ],
  },
  render: args => ({
    components: { EntitySelect },
    setup: () => ({ args }),
    template: `
      <div class="h-80 w-60 bg-base-200">
        <div class="my-1 border-y border-base-300 p-2">
          <EntitySelect v-bind="args">
            <template #trigger><span class="min-w-0 text-start"><span class="block truncate font-mono text-xs font-medium">acme-prod</span></span></template>
            <template #end><span class="badge badge-sm">Production</span></template>
            <template #actions>
              <li><a href="#settings"><Icon name="lucide:settings" size="16" class="shrink-0" />Project settings</a></li>
              <li><button type="button"><Icon name="lucide:plus" size="16" class="shrink-0" />New project</button></li>
            </template>
          </EntitySelect>
        </div>
      </div>`,
  }),
};
