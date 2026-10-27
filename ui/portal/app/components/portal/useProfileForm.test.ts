import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';

import { ProfileSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useAccountStore } from '~/stores/account';
import { useProfileForm } from './useProfileForm';

const ana = create(ProfileSchema, { id: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'ana@acme.test', name: 'Ana' });

describe('useProfileForm', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(async () => {
    setActivePinia(createPinia());
    client = {
      getProfile: vi.fn().mockResolvedValue({ profile: ana, signOutToken: 'tok', currentSessionId: 'M8vBq3XzL1kRw6PtN9cJf' }),
      updateProfile: vi.fn(async (req: { name: string }) => ({ profile: create(ProfileSchema, { ...ana, name: req.name }) })),
    };
    setClient('$accountClient', client);
    await useAccountStore().load();
  });

  it('starts from the stored name and leaves the store holding no form state', () => {
    const f = useProfileForm();
    expect(f.name.value).toBe('Ana');
    expect(Object.keys(useAccountStore().$state)).not.toContain('saved');
  });

  it('saves a trimmed name into the store and keeps the saved note', async () => {
    const f = useProfileForm();
    f.name.value = '  Ana Putri ';
    expect(await f.save()).toBe(true);
    expect(client.updateProfile).toHaveBeenLastCalledWith({ name: 'Ana Putri' });
    expect(useAccountStore().profile?.name).toBe('Ana Putri');
    await nextTick();
    expect(f.name.value).toBe('Ana Putri');
    expect(f.saved.value).toBe(true);
  });

  it('clears the saved note once the name changes again', async () => {
    const f = useProfileForm();
    f.name.value = 'Ana Putri';
    await f.save();
    await nextTick();
    f.name.value = 'Ana P.';
    await nextTick();
    expect(f.saved.value).toBe(false);
  });

  it('refuses a name too long and shows a server refusal', async () => {
    setKnownKeys('apperror.GEN900');
    const f = useProfileForm();
    f.name.value = 'x'.repeat(201);
    expect(await f.save()).toBe(false);
    expect(f.fieldErrors.value.name).toBeTruthy();
    client.updateProfile!.mockRejectedValueOnce(connectError('GEN900'));
    f.name.value = '';
    expect(await f.save()).toBe(false);
    expect(f.error.value?.message).toBe('apperror.GEN900');
  });

  it('ignores a save while one runs and keeps the profile when the answer has none', async () => {
    let finish!: (v: unknown) => void;
    client.updateProfile!.mockReturnValueOnce(new Promise((resolve) => {
      finish = resolve;
    }));
    const f = useProfileForm();
    const first = f.save();
    expect(await f.save()).toBe(false);
    finish({});
    expect(await first).toBe(true);
    expect(useAccountStore().profile?.name).toBe('Ana');
  });

  it('follows the stored profile, and starts empty without one', async () => {
    const f = useProfileForm();
    const account = useAccountStore();
    account.replaceProfile(create(ProfileSchema, { ...ana, name: 'Ana Putri' }));
    await nextTick();
    expect(f.name.value).toBe('Ana Putri');
    account.profile = null;
    await nextTick();
    expect(f.name.value).toBe('');
    expect(useProfileForm().name.value).toBe('');
  });
});
