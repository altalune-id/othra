import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ProfileSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useAccountStore } from './account';

const ana = create(ProfileSchema, { id: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'ana@acme.test', name: 'Ana' });

describe('useAccountStore', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    setActivePinia(createPinia());
    client = {
      getProfile: vi.fn().mockResolvedValue({ profile: ana, signOutToken: 'tok', currentSessionId: 'M8vBq3XzL1kRw6PtN9cJf' }),
    };
    setClient('$accountClient', client);
  });

  it('loads the profile once', async () => {
    const s = useAccountStore();
    await s.ensureLoaded();
    await s.ensureLoaded();
    expect(client.getProfile).toHaveBeenCalledTimes(1);
    expect(s.profile?.email).toBe('ana@acme.test');
    expect(s.signOutToken).toBe('tok');
    expect(s.currentSessionId).toBe('M8vBq3XzL1kRw6PtN9cJf');
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.ACC001');
    client.getProfile!.mockRejectedValueOnce(connectError('ACC001'));
    const s = useAccountStore();
    await s.load();
    expect(s.loadError?.message).toBe('apperror.ACC001');
    expect(s.loaded).toBe(false);
  });

  it('replaces the profile only with a real one', async () => {
    const s = useAccountStore();
    await s.load();
    s.replaceProfile(undefined);
    expect(s.profile?.name).toBe('Ana');
    s.replaceProfile(create(ProfileSchema, { ...ana, name: 'Ana Putri' }));
    expect(s.profile?.name).toBe('Ana Putri');
  });

  it('holds no profile when the answer has none', async () => {
    client.getProfile!.mockResolvedValueOnce({ signOutToken: 'tok', currentSessionId: 'M8vBq3XzL1kRw6PtN9cJf' });
    const s = useAccountStore();
    await s.load();
    expect(s.profile).toBeNull();
    expect(s.loaded).toBe(true);
  });
});
