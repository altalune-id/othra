import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

import { setClient } from '#test/nuxt-globals';
import { useAuthStore } from '~/stores/auth';
import { canEditProject, projectGuard, useCanEditProject, useProjectGuard } from './useProjectAccess';

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

describe('projectGuard', () => {
  const admin = { permissions: ['project:write'] };
  const member = { permissions: ['project:read'] };

  it('lets an admin change everything on an ordinary project', () => {
    expect(projectGuard(admin, { system: false })).toEqual({ canWrite: true, canConfigure: true, system: false });
    expect(projectGuard(admin, undefined)).toEqual({ canWrite: true, canConfigure: true, system: false });
  });

  it('keeps the console project to what Othra allows, even for an admin', () => {
    expect(projectGuard(admin, { system: true })).toEqual({ canWrite: true, canConfigure: false, system: true });
  });

  it('gives a member nothing to change on either', () => {
    expect(projectGuard(member, { system: false })).toEqual({ canWrite: false, canConfigure: false, system: false });
    expect(projectGuard(member, { system: true })).toEqual({ canWrite: false, canConfigure: false, system: true });
  });
});

describe('useProjectGuard', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('follows the signed-in person and the project', async () => {
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue({ source: 'local', userId: 'F5pLm2XwQ8kRt3VnZ7bYc', permissions: ['project:write'] }) });
    const project = ref<{ system?: boolean }>({ system: true });
    const guard = useProjectGuard(() => project.value);
    await useAuthStore().load();
    expect(guard.value.canConfigure).toBe(false);
    project.value = { system: false };
    expect(guard.value.canConfigure).toBe(true);
  });
});
