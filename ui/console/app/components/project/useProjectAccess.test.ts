import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { setClient } from '#test/nuxt-globals';
import { useAuthStore } from '~/stores/auth';
import { canEditProject, useCanEditProject } from './useProjectAccess';

describe('canEditProject', () => {
  it('lets an owner, an admin and an operator edit project settings', () => {
    expect(canEditProject({ permissions: ['*'] })).toBe(true);
    expect(canEditProject({ permissions: ['org:read', 'project:*'] })).toBe(true);
    expect(canEditProject({ permissions: ['project:write'] })).toBe(true);
    expect(canEditProject({ operator: true, permissions: [] })).toBe(true);
  });

  it('keeps a member, who only reads projects, on read-only settings', () => {
    expect(canEditProject({ permissions: ['org:read', 'project:read', 'webhook:*'] })).toBe(false);
    expect(canEditProject({ permissions: [] })).toBe(false);
  });

  it('leaves the decision to the server when nobody is signed in', () => {
    expect(canEditProject(null)).toBe(true);
  });
});

describe('useCanEditProject', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('follows the signed-in person', async () => {
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue({ source: 'local', userId: 'u1', permissions: ['project:read'] }) });
    const canEdit = useCanEditProject();
    await useAuthStore().load();
    expect(canEdit.value).toBe(false);
  });
});
