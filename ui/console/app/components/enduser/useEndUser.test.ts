import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { tokenWhen } from '#base/lib/account/views';
import { ConnectedAppSchema, PersonalTokenSchema, PersonalTokenState, SessionSchema, SignInMethodKind, SignInMethodSchema } from '#gen/account/v1/account_pb';
import { EndUserSchema } from '#gen/enduser/v1/enduser_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useEndUser } from './useEndUser';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const endUserId = 'dPm7fmQRJKPVw8PxrgdrN';
const ids = { projectId, endUserId };

describe('useEndUser', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    client = {
      getEndUser: vi.fn().mockResolvedValue({
        endUser: create(EndUserSchema, { id: endUserId, email: 'ana@acme.test' }),
        methods: [create(SignInMethodSchema, { kind: SignInMethodKind.EMAIL_LINK, linked: true, email: 'ana@acme.test' })],
        sessions: [create(SessionSchema, { id: 'Q7mZp2LkV9xRt4NbW8cYd' })],
        apps: [create(ConnectedAppSchema, { clientId: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Web' })],
        tokens: [create(PersonalTokenSchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', state: PersonalTokenState.ACTIVE })],
      }),
      revokeEndUserSession: vi.fn().mockResolvedValue({}),
      disconnectEndUserApp: vi.fn().mockResolvedValue({}),
      revokeEndUserToken: vi.fn().mockResolvedValue({}),
    };
    setClient('$endUserClient', client);
  });

  it('loads the detail through the shared views', async () => {
    const u = useEndUser(projectId, endUserId);
    await u.load();
    expect(client.getEndUser).toHaveBeenCalledWith(ids);
    expect(u.user.value?.email).toBe('ana@acme.test');
    expect(u.methods.value.map(m => m.key)).toEqual(['email_link']);
    expect(u.sessions.value[0]!.id).toBe('Q7mZp2LkV9xRt4NbW8cYd');
    expect(u.apps.value[0]!.name).toBe('Web');
    expect(u.tokens.value[0]!.status).toBe('active');
  });

  it('tells not found apart from other load errors', async () => {
    setKnownKeys('apperror.EUS001', 'apperror.GEN900');
    client.getEndUser!.mockRejectedValueOnce(connectError('EUS001')).mockRejectedValueOnce(connectError('GEN900')).mockResolvedValueOnce({ methods: [], sessions: [], apps: [], tokens: [] });
    const u = useEndUser(projectId, endUserId);
    await u.load();
    expect(u.notFound.value).toBe(true);
    await u.load();
    expect(u.notFound.value).toBe(false);
    expect(u.loadError.value?.message).toBe('apperror.GEN900');
    await u.load();
    expect(u.notFound.value).toBe(true);
  });

  it('is ready only with a user and no load error, so a failed load never reads as "nothing to revoke"', async () => {
    setKnownKeys('apperror.GEN900');
    const u = useEndUser(projectId, endUserId);
    expect(u.ready.value).toBe(false);
    await u.load();
    expect(u.ready.value).toBe(true);
    client.getEndUser!.mockRejectedValueOnce(connectError('GEN900'));
    const again = u.load();
    expect(u.loading.value).toBe(true);
    await again;
    expect(u.loading.value).toBe(false);
    expect(u.ready.value).toBe(false);
    expect(u.notFound.value).toBe(false);
    await u.load();
    expect(u.ready.value).toBe(true);
  });

  it('revokes a session, an app and a token', async () => {
    const u = useEndUser(projectId, endUserId);
    await u.load();
    u.endSession.ask(u.sessions.value[0]!);
    await u.endSession.confirm();
    expect(client.revokeEndUserSession).toHaveBeenCalledWith({ ...ids, sessionId: 'Q7mZp2LkV9xRt4NbW8cYd' });
    expect(u.sessions.value).toEqual([]);
    u.disconnect.ask(u.apps.value[0]!);
    await u.disconnect.confirm();
    expect(client.disconnectEndUserApp).toHaveBeenCalledWith({ ...ids, clientId: 'dPm7fmQRJKPVw8PxrgdrN' });
    expect(u.apps.value).toEqual([]);
    u.revokeToken.ask(u.tokens.value[0]!);
    await u.revokeToken.confirm();
    expect(client.revokeEndUserToken).toHaveBeenCalledWith({ ...ids, tokenId: 'K9tWq4ZbN2xLm7VcR5pHs' });
    expect(u.tokens.value[0]!.status).toBe('revoked');
    expect(u.tokens.value[0]!.revokedAt, 'the row reads "Revoked <date>", never a bare "Revoked"').toBeInstanceOf(Date);
    expect(tokenWhen(u.tokens.value[0]!).date).toBe(u.tokens.value[0]!.revokedAt);
  });
});
