import type { Meta, StoryObj } from '@storybook/vue3-vite';
import { ref } from 'vue';
import ConfirmModal from './ConfirmModal.vue';

function handle(error: { message: string, reference?: string } | null = null, busy = false) {
  return {
    target: ref<unknown>({ id: 'tok_8Jq2xKpL4mNa' }),
    busy: ref(busy),
    error: ref(error),
    cancel: () => {},
    confirm: async () => true,
  };
}

const meta = {
  title: 'UI/ConfirmModal',
  component: ConfirmModal,
  args: {
    handle: handle(),
    title: 'Revoke this token?',
    body: 'Anything using deploy-bot stops working at once. This can\'t be undone.',
    confirmLabel: 'Revoke',
    cancelLabel: 'Cancel',
  },
} satisfies Meta<typeof ConfirmModal>;

export default meta;
type Story = StoryObj<typeof meta>;

function withHandle(error: { message: string, reference?: string } | null = null, busy = false): Story['render'] {
  return args => ({
    components: { ConfirmModal },
    setup: () => ({ args, state: handle(error, busy) }),
    template: '<ConfirmModal v-bind="args" :handle="state" />',
  });
}

export const Default: Story = { render: withHandle() };
export const Refused: Story = { render: withHandle({ message: 'Couldn\'t revoke the token.', reference: 'Ref: req-1' }) };
export const Busy: Story = { render: withHandle(null, true) };
