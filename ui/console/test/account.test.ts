// @vitest-environment happy-dom
import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { WhoamiResponseSchema } from '#gen/auth/v1/auth_pb';
import { mountPage } from '#test/mount-page';
import { setClient } from '#test/nuxt-globals';
import Account from '~/pages/account/index.vue';

function signedIn(passwords: boolean) {
  setClient('$authClient', {
    whoami: vi.fn().mockResolvedValue(create(WhoamiResponseSchema, { source: 'oidc', userId: 'V3StGXR8Z5jdHj6BmyT2a', name: 'Ana', email: 'ana@example.com', passwords })),
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe('account page', () => {
  it('hides Change password when whoami says passwords is false', async () => {
    signedIn(false);
    const w = await mountPage(Account);
    expect(w.text()).toContain('ana@example.com');
    expect(w.text()).not.toContain('profile.password_title');
  });

  it('shows it when whoami says passwords is true', async () => {
    signedIn(true);
    const w = await mountPage(Account);
    expect(w.text()).toContain('profile.password_title');
  });
});
