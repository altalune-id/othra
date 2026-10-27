import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { APIKeySchema } from '#gen/apikey/v1/apikey_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useAPIKeysStore } from '~/stores/apiKeys';
import { createdKeyState, usePersonalKeys } from './usePersonalKeys';

const acme = { id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme' };
const globex = { id: 'YeDxg2kMYznPJJG3wVmjj', slug: 'globex', name: 'Globex' };

function key(orgId: string, hint: string, extra: Record<string, unknown> = {}) {
  return create(APIKeySchema, { orgId, secretHint: hint, kind: 'personal', name: hint, scopes: ['webhook:read'], ...extra });
}
const at = (iso: string) => timestampFromDate(new Date(iso));

describe('usePersonalKeys', () => {
  let listAPIKeys: ReturnType<typeof vi.fn>;
  let revokeAPIKey: ReturnType<typeof vi.fn>;
  let listOrgs: ReturnType<typeof vi.fn>;
  let whoami: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    whoami = vi.fn().mockResolvedValue({ source: 'local', userId: 'u1', activeOrgId: acme.id, permissions: ['*'] });
    listOrgs = vi.fn().mockResolvedValue({ orgs: [acme, globex] });
    listAPIKeys = vi.fn(async ({ orgId }: { orgId: string }) => ({
      keys: orgId === acme.id
        ? [key(acme.id, 'aaaa', { createdAt: at('2026-09-01T00:00:00Z') }), key(acme.id, 'oorg', { kind: 'org' })]
        : [key(globex.id, 'gggg')],
    }));
    revokeAPIKey = vi.fn().mockResolvedValue({});
    setClient('$authClient', { whoami });
    setClient('$orgClient', { listOrgs });
    setClient('$apiKeyClient', { listAPIKeys, revokeAPIKey });
  });

  it('lists the viewer\'s personal keys across every org, with the org name', async () => {
    const keys = usePersonalKeys();
    expect(keys.loaded.value).toBe(false);
    await keys.load();
    expect(listAPIKeys).toHaveBeenCalledWith({ orgId: acme.id });
    expect(listAPIKeys).toHaveBeenCalledWith({ orgId: globex.id });
    expect(keys.rows.value.map(r => [r.orgName, r.secretHint, r.scopeLabels])).toEqual([
      ['Acme', 'aaaa', ['scope.webhook_read']],
      ['Globex', 'gggg', ['scope.webhook_read']],
    ]);
    expect(keys.loaded.value).toBe(true);
    expect(keys.oneOrg.value).toBe(false);
    expect(keys.failed.value).toEqual([]);
  });

  it('keeps its own state and leaves the org settings store alone', async () => {
    const keys = usePersonalKeys();
    await keys.load();
    expect(useAPIKeysStore().items).toEqual([]);
  });

  it('sorts by org name, then live before revoked, then newest first', async () => {
    listOrgs.mockResolvedValue({ orgs: [globex, acme] });
    listAPIKeys.mockImplementation(async ({ orgId }: { orgId: string }) => ({
      keys: orgId === acme.id
        ? [
            key(acme.id, 'old1', { createdAt: at('2026-01-01T00:00:00Z') }),
            key(acme.id, 'rev1', { createdAt: at('2026-09-01T00:00:00Z'), revokedAt: at('2026-09-02T00:00:00Z') }),
            key(acme.id, 'new1', { createdAt: at('2026-08-01T00:00:00Z') }),
            key(acme.id, 'nod1'),
          ]
        : [key(globex.id, 'gggg')],
    }));
    const keys = usePersonalKeys();
    await keys.load();
    expect(keys.rows.value.map(r => r.secretHint)).toEqual(['new1', 'old1', 'nod1', 'rev1', 'gggg']);
  });

  it('shows the keys of the orgs that loaded and names the ones that failed, then retries one', async () => {
    listAPIKeys.mockImplementation(async ({ orgId }: { orgId: string }) => {
      if (orgId === globex.id) {
        throw connectError('GEN900');
      }
      return { keys: [key(acme.id, 'aaaa')] };
    });
    const keys = usePersonalKeys();
    await keys.load();
    expect(keys.rows.value.map(r => r.secretHint)).toEqual(['aaaa']);
    expect(keys.failed.value).toEqual([{ id: globex.id, name: 'Globex' }]);

    listAPIKeys.mockImplementation(async () => ({ keys: [key(globex.id, 'gggg')] }));
    await keys.retry(globex.id);
    expect(keys.failed.value).toEqual([]);
    expect(keys.rows.value.map(r => r.secretHint)).toEqual(['aaaa', 'gggg']);
  });

  it('names an org once when its retry fails again, and treats a missing list as empty', async () => {
    listAPIKeys.mockImplementation(async ({ orgId }: { orgId: string }) => {
      if (orgId === globex.id) {
        throw new Error('down');
      }
      return {};
    });
    const keys = usePersonalKeys();
    await keys.load();
    await keys.retry(globex.id);
    expect(keys.failed.value).toEqual([{ id: globex.id, name: 'Globex' }]);
    expect(keys.rows.value).toEqual([]);
  });

  it('revokes in an org whose keys never loaded without inventing rows', async () => {
    listAPIKeys.mockRejectedValue(new Error('down'));
    const keys = usePersonalKeys();
    await keys.load();
    await keys.revoke(globex.id, 'gone');
    expect(revokeAPIKey).toHaveBeenCalledWith({ orgId: globex.id, hint: 'gone' });
    expect(keys.rows.value).toEqual([]);
  });

  it('reports one org and hides the org column then', async () => {
    listOrgs.mockResolvedValue({ orgs: [acme] });
    const keys = usePersonalKeys();
    await keys.load();
    expect(keys.oneOrg.value).toBe(true);
    expect(listAPIKeys).toHaveBeenCalledTimes(1);
  });

  it('loads the keys of someone who signed in with Continue with Othra', async () => {
    whoami.mockResolvedValue({ source: 'oidc', userId: 'u1', activeOrgId: acme.id, permissions: ['*'] });
    const keys = usePersonalKeys();
    await keys.load();
    expect(keys.signedIn.value).toBe(true);
    expect(listAPIKeys).toHaveBeenCalled();
  });

  it('loads nothing for a caller who is not a signed-in person', async () => {
    whoami.mockResolvedValue({ source: 'apikey', permissions: [] });
    const keys = usePersonalKeys();
    await keys.load();
    expect(keys.signedIn.value).toBe(false);
    expect(listAPIKeys).not.toHaveBeenCalled();
    expect(keys.rows.value).toEqual([]);
    expect(keys.loaded.value).toBe(true);
  });

  it('revokes a key in its own org, by hint', async () => {
    listAPIKeys.mockImplementation(async ({ orgId }: { orgId: string }) => ({ keys: [key(orgId, 'same')] }));
    const keys = usePersonalKeys();
    await keys.load();
    await keys.revoke(globex.id, 'same');
    expect(revokeAPIKey).toHaveBeenCalledWith({ orgId: globex.id, hint: 'same' });
    const byOrg = Object.fromEntries(keys.rows.value.map(r => [r.orgId, r.revokedAt]));
    expect(byOrg[acme.id]).toBeUndefined();
    expect(byOrg[globex.id]).toBeInstanceOf(Date);
    expect(keys.error.value).toBeNull();
  });

  it('shows revoke errors and keeps the key live', async () => {
    setKnownKeys('apperror.GEN900');
    revokeAPIKey.mockRejectedValueOnce(connectError('GEN900')).mockRejectedValueOnce(new Error('boom'));
    const keys = usePersonalKeys();
    await keys.load();
    await keys.revoke(acme.id, 'aaaa');
    expect(keys.error.value).toEqual({ message: 'apperror.GEN900', reference: 'error.ref|{"id":"req-1"}' });
    await keys.revoke(acme.id, 'aaaa');
    expect(keys.error.value).toEqual({ message: 'error.revoke_apikey' });
    expect(keys.rows.value.find(r => r.secretHint === 'aaaa')?.revokedAt).toBeUndefined();
  });

  it('shows the one-time secret handed over by the create page until it is dismissed', async () => {
    const keys = usePersonalKeys();
    expect(keys.created.value).toBeNull();
    createdKeyState().value = { plaintext: 'othra_new', snippet: '{}' };
    expect(keys.created.value).toEqual({ plaintext: 'othra_new', snippet: '{}' });
    keys.dismiss();
    expect(keys.created.value).toBeNull();
    expect(createdKeyState().value).toBeNull();
  });
});
