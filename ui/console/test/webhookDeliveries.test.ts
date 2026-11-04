// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WebhookAttemptSchema, WebhookDeliverySchema } from '#gen/webhook/v1/webhook_pb';
import { mountPage } from '#test/mount-page';
import { deferred, setClient, setRoute } from '#test/nuxt-globals';
import Deliveries from '~/pages/o/[org]/projects/[project]/webhooks/[webhook].vue';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

const orgId = 'B6rTn1YkM8wQz3XpL5vGd';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';

beforeEach(() => {
  setActivePinia(createPinia());
  useOrgsStore().items = [{ id: orgId, slug: 'acme', name: 'Acme', role: 'owner', permissions: [] }];
  useProjectsStore().items = [{ id: projectId, orgId, slug: 'web', name: 'Web' }];
  setRoute({ path: '/o/acme/projects/web/webhooks/H3nVb8XqT1wLp6RkZ2mJc', params: { org: 'acme', project: 'web', webhook: 'H3nVb8XqT1wLp6RkZ2mJc' } });
});

describe('webhook deliveries page', () => {
  it('shows loading, not the empty state, while the first page loads', async () => {
    const pending = deferred<{ deliveries: unknown[] }>();
    setClient('$webhookClient', { listWebhookDeliveries: vi.fn().mockReturnValue(pending.promise) });
    const w = await mountPage(Deliveries);
    expect(w.find('[role="status"]').text()).toContain('common.loading');
    expect(w.text()).not.toContain('webhooks.no_deliveries');
    pending.resolve({ deliveries: [] });
    await flushPromises();
    expect(w.text()).toContain('webhooks.no_deliveries');
  });

  it('translates delivery statuses and shows No response for an attempt without a status code', async () => {
    const delivery = create(WebhookDeliverySchema, { id: 'D1', eventType: 'member.added', status: 'failed', attempt: 2 });
    setClient('$webhookClient', {
      listWebhookDeliveries: vi.fn().mockResolvedValue({ deliveries: [delivery] }),
      listWebhookDeliveryAttempts: vi.fn().mockResolvedValue({ attempts: [create(WebhookAttemptSchema, { attempt: 1, statusCode: 0, error: 'dial tcp: timeout' })] }),
    });
    const w = await mountPage(Deliveries, { attachTo: document.body });
    await flushPromises();
    expect(w.get('tbody').text()).toContain('webhooks.status_failed');
    expect(w.get('tbody').text()).not.toMatch(/\bfailed\b/);
    await w.findAll('tbody button').find(b => b.text().includes('webhooks.view_attempts'))!.trigger('click');
    await flushPromises();
    const dialog = document.querySelector('dialog[open]')!;
    expect(dialog.textContent).toContain('webhooks.no_response');
    expect(dialog.textContent).not.toMatch(/(^|\s)0(\s|$)/);
    w.unmount();
  });
});
