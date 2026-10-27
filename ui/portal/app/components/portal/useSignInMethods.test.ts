import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { SignInMethodKind, SignInMethodSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useSignInMethods } from './useSignInMethods';

const google = create(SignInMethodSchema, { kind: SignInMethodKind.GOOGLE, linked: true, identityId: 'J2xKv7NqP4wRt9LmZ6cBh', email: 'ana.s@gmail.com', canRemove: true });
const email = create(SignInMethodSchema, { kind: SignInMethodKind.EMAIL_LINK, linked: true, email: 'ana@acme.test' });

describe('useSignInMethods', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    client = {
      getSignInMethods: vi.fn().mockResolvedValue({ methods: [google, email], linkToken: 'tok' }),
      unlinkIdentity: vi.fn().mockResolvedValue({}),
    };
    setClient('$accountClient', client);
  });

  it('loads rows and the link token', async () => {
    const m = useSignInMethods();
    await m.load();
    expect(m.rows.value.map(r => r.key)).toEqual(['email_link', 'google']);
    expect(m.linkToken.value).toBe('tok');
    expect(m.loading.value).toBe(false);
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.getSignInMethods!.mockRejectedValueOnce(connectError('GEN900'));
    const m = useSignInMethods();
    await m.load();
    expect(m.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('disconnects Google by its identity, then reloads', async () => {
    const m = useSignInMethods();
    await m.load();
    m.askUnlink(m.rows.value[1]!);
    expect(await m.unlink.confirm()).toBe(true);
    expect(client.unlinkIdentity).toHaveBeenCalledWith({ identityId: 'J2xKv7NqP4wRt9LmZ6cBh' });
    expect(m.unlinked.value).toBe(true);
    expect(client.getSignInMethods).toHaveBeenCalledTimes(2);
  });

  it('never asks to disconnect the only way to sign in', async () => {
    client.getSignInMethods!.mockResolvedValue({ methods: [create(SignInMethodSchema, { ...google, canRemove: false }), email], linkToken: 'tok' });
    const m = useSignInMethods();
    await m.load();
    m.askUnlink(m.rows.value[1]!);
    expect(m.unlink.target.value).toBeNull();
  });
});
