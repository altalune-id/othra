import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { setClient } from '#test/nuxt-globals';
import { useActiveOrg } from './useActiveOrg';

function setup(me: Record<string, unknown>) {
  const setActiveOrg = vi.fn().mockResolvedValue({ me: { ...me, activeOrgSlug: 'beta' } });
  setClient('$authClient', { whoami: vi.fn().mockResolvedValue(me), setActiveOrg });
  setClient('$orgClient', {
    listOrgs: vi.fn().mockResolvedValue({ orgs: [{ id: 'acmeId', slug: 'acme', name: 'Acme' }, { id: 'betaId', slug: 'beta', name: 'Beta' }] }),
  });
  return setActiveOrg;
}

describe('useActiveOrg', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('switches a person to the org they open', async () => {
    const setActiveOrg = setup({ source: 'local', activeOrgSlug: 'acme' });
    await useActiveOrg().enter('beta');
    expect(setActiveOrg).toHaveBeenCalledWith({ orgId: 'betaId' });
  });

  it('does nothing when the org is already active', async () => {
    const setActiveOrg = setup({ source: 'local', activeOrgSlug: 'beta' });
    await useActiveOrg().enter('beta');
    expect(setActiveOrg).not.toHaveBeenCalled();
  });

  it('does nothing for an org the person does not have (the page answers not found)', async () => {
    const setActiveOrg = setup({ source: 'local', activeOrgSlug: 'acme' });
    await useActiveOrg().enter('foreign');
    expect(setActiveOrg).not.toHaveBeenCalled();
  });

  it('never switches a key or basic auth, which reach orgs without it', async () => {
    for (const me of [{ source: 'apikey' }, { source: 'basic' }]) {
      setActivePinia(createPinia());
      const setActiveOrg = setup(me);
      await useActiveOrg().enter('beta');
      expect(setActiveOrg).not.toHaveBeenCalled();
    }
  });

  it('switches an operator to an org they belong to, so project pages there load', async () => {
    const setActiveOrg = setup({ source: 'local', operator: true, activeOrgSlug: 'acme' });
    await useActiveOrg().enter('beta');
    expect(setActiveOrg).toHaveBeenCalledWith({ orgId: 'betaId' });
  });

  it('keeps an operator in an org they do not belong to when the switch is refused', async () => {
    setup({ source: 'local', operator: true, activeOrgSlug: 'acme' });
    const setActiveOrg = vi.fn().mockRejectedValue(new Error('not found'));
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue({ source: 'local', operator: true, activeOrgSlug: 'acme' }), setActiveOrg });
    await expect(useActiveOrg().enter('beta')).resolves.toBeUndefined();
    expect(setActiveOrg).toHaveBeenCalledWith({ orgId: 'betaId' });
  });

  it('does nothing with nobody signed in', async () => {
    setClient('$authClient', { whoami: vi.fn().mockRejectedValue(new Error('401')), setActiveOrg: vi.fn() });
    setClient('$orgClient', { listOrgs: vi.fn().mockResolvedValue({ orgs: [] }) });
    await expect(useActiveOrg().enter('beta')).resolves.toBeUndefined();
  });

  it('lets a failed switch surface', async () => {
    setup({ source: 'local', activeOrgSlug: 'acme' });
    setClient('$authClient', {
      whoami: vi.fn().mockResolvedValue({ source: 'local', activeOrgSlug: 'acme' }),
      setActiveOrg: vi.fn().mockRejectedValue(new Error('boom')),
    });
    await expect(useActiveOrg().enter('beta')).rejects.toThrow('boom');
  });
});
