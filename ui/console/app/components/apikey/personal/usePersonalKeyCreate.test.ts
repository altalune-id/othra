import { timestampDate } from '@bufbuild/protobuf/wkt';
import scopes from '@shared/scope/scopes.json';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { connectError, navigateTo, runtimeConfig, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { mintableForMe } from './grants';
import { mcpSnippet } from './mcpSnippet';
import { usePersonalKeyCreate } from './usePersonalKeyCreate';
import { createdKeyState } from './usePersonalKeys';

const acme = { id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme', role: 'admin', permissions: ['webhook:*', 'member:write'] };
const globex = { id: 'YeDxg2kMYznPJJG3wVmjj', slug: 'globex', name: 'Globex', role: 'member', permissions: ['org:read', 'project:read', 'webhook:*', 'member:read', 'apikey:read'] };
const initech = { id: 'KdrF3t3FHtppBPmMgj4sy', slug: 'initech', name: 'Initech', role: '', permissions: [] };
const now = new Date('2026-10-01T00:00:00Z');

describe('usePersonalKeyCreate', () => {
  let listOrgs: ReturnType<typeof vi.fn>;
  let listProjects: ReturnType<typeof vi.fn>;
  let createAPIKey: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.useFakeTimers();
    vi.setSystemTime(now);
    vi.stubGlobal('window', { location: { origin: 'https://console.test' } });
    listOrgs = vi.fn().mockResolvedValue({ orgs: [acme, globex] });
    listProjects = vi.fn(async ({ orgId }: { orgId: string }) => ({
      projects: [{ id: orgId === acme.id ? 'x6MH9WPWjktMwCBKHy4pF' : 'KdrF3t3FHtppBPmMgj4sy', orgId, slug: 'p', name: `${orgId} project` }],
    }));
    createAPIKey = vi.fn().mockResolvedValue({ plaintext: 'othra_new' });
    setClient('$authClient', { whoami: vi.fn().mockResolvedValue({ source: 'local', userId: 'u1', activeOrgId: acme.id, permissions: ['webhook:*', 'member:write'] }) });
    setClient('$orgClient', { listOrgs });
    setClient('$projectClient', { listProjects });
    setClient('$apiKeyClient', { createAPIKey });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts with no org when there are several, so the person picks one', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    expect(form.oneOrg.value).toBe(false);
    expect(form.orgId.value).toBe('');
    expect(form.orgOptions.value).toEqual([{ value: acme.id, label: 'Acme' }, { value: globex.id, label: 'Globex' }]);
    expect(form.mintable.value).toEqual([]);
    expect(form.expiresInDays.value).toBe(30);
    expect(form.expiryOptions.map(o => o.days)).toEqual([7, 30, 90, 365]);
    expect(listProjects).not.toHaveBeenCalled();
  });

  it('preselects the only org and loads its projects', async () => {
    listOrgs.mockResolvedValue({ orgs: [acme] });
    const form = usePersonalKeyCreate();
    await form.load();
    expect(form.oneOrg.value).toBe(true);
    expect(form.orgId.value).toBe(acme.id);
    expect(form.orgName.value).toBe('Acme');
    expect(form.projects.value).toEqual([{ id: 'x6MH9WPWjktMwCBKHy4pF', name: `${acme.id} project` }]);
  });

  it('offers the active org only the scopes the viewer\'s role holds', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(acme.id);
    expect(form.mintable.value).toEqual([
      { name: 'webhook:read', label: 'scope.webhook_read' },
      { name: 'webhook:write', label: 'scope.webhook_write' },
    ]);
  });

  it('offers another org only the scopes the viewer\'s role there holds', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(globex.id);
    expect(form.mintable.value.map(s => s.name)).toEqual(mintableForMe(scopes, globex.permissions).map(s => s.name));
    expect(form.mintable.value.map(s => s.name)).not.toContain('member:write');
  });

  it('offers every scope a personal key may hold when the org carries no role, and leaves the check to the server', async () => {
    listOrgs.mockResolvedValue({ orgs: [acme, initech] });
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(initech.id);
    expect(form.mintable.value.map(s => s.name)).toEqual(scopes.filter(s => !s.personOnly).map(s => s.name));
  });

  it('resets projects and scopes when the org changes, and fetches each org\'s projects once', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(acme.id);
    form.scopes.value = ['webhook:read'];
    form.allProjects.value = false;
    form.projectIds.value = ['x6MH9WPWjktMwCBKHy4pF'];

    await form.selectOrg(globex.id);
    expect(form.scopes.value).toEqual([]);
    expect(form.allProjects.value).toBe(true);
    expect(form.projectIds.value).toEqual([]);
    expect(form.projects.value.map(p => p.id)).toEqual(['KdrF3t3FHtppBPmMgj4sy']);

    await form.selectOrg(acme.id);
    expect(listProjects).toHaveBeenCalledTimes(2);
    expect(form.projects.value.map(p => p.id)).toEqual(['x6MH9WPWjktMwCBKHy4pF']);
  });

  it('takes the scopes from the org even when whoami fails, and treats a missing project list as empty', async () => {
    setClient('$authClient', { whoami: vi.fn().mockRejectedValue(new Error('down')) });
    listProjects.mockResolvedValue({});
    const form = usePersonalKeyCreate();
    await form.load();
    expect(form.orgName.value).toBe('');
    await form.selectOrg(acme.id);
    expect(form.mintable.value.map(s => s.name)).toEqual(['webhook:read', 'webhook:write']);
    expect(form.projects.value).toEqual([]);
  });

  it('shows no projects when they fail to load', async () => {
    listProjects.mockRejectedValue(new Error('down'));
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(acme.id);
    expect(form.projects.value).toEqual([]);
  });

  it('creates the key in the chosen org, hands the secret to the list and goes back', async () => {
    runtimeConfig.public.apiUrl = '/base/api';
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(globex.id);
    form.name.value = '  Laptop  ';
    form.scopes.value = ['webhook:read'];
    form.expiresInDays.value = 7;
    await form.submit();

    const req = createAPIKey.mock.calls[0]![0];
    expect(req).toMatchObject({ orgId: globex.id, kind: 'personal', name: 'Laptop', scopes: ['webhook:read'], allProjects: true, projectIds: [] });
    expect(timestampDate(req.expiresAt)).toEqual(new Date(now.getTime() + 7 * 86_400_000));
    expect(createdKeyState().value).toEqual({ plaintext: 'othra_new', snippet: mcpSnippet('othra', 'https://console.test/base/mcp', 'othra_new') });
    expect(navigateTo).toHaveBeenCalledWith('/account/api-keys');
    expect(form.busy.value).toBe(false);
  });

  it('sends only the chosen projects when not all projects are picked', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(acme.id);
    form.name.value = 'CI';
    form.allProjects.value = false;
    form.projectIds.value = ['x6MH9WPWjktMwCBKHy4pF'];
    await form.submit();
    expect(createAPIKey.mock.calls[0]![0]).toMatchObject({ allProjects: false, projectIds: ['x6MH9WPWjktMwCBKHy4pF'] });
  });

  it('does nothing without a name or an org', async () => {
    const form = usePersonalKeyCreate();
    await form.load();
    form.name.value = 'Laptop';
    expect(form.canSubmit.value).toBe(false);
    await form.submit();
    await form.selectOrg(acme.id);
    form.name.value = '   ';
    expect(form.canSubmit.value).toBe(false);
    await form.submit();
    expect(createAPIKey).not.toHaveBeenCalled();
  });

  it('shows create errors and stays on the page', async () => {
    setKnownKeys('apperror.GEN007');
    createAPIKey.mockRejectedValueOnce(connectError('GEN007', { permission: 'webhook:write' })).mockRejectedValueOnce(new Error('boom'));
    const form = usePersonalKeyCreate();
    await form.load();
    await form.selectOrg(acme.id);
    form.name.value = 'Laptop';
    await form.submit();
    expect(form.error.value).toEqual({ message: 'apperror.GEN007|{"permission":"scope.webhook_write"}', reference: 'error.ref|{"id":"req-1"}' });
    await form.submit();
    expect(form.error.value).toEqual({ message: 'error.create_apikey' });
    expect(navigateTo).not.toHaveBeenCalled();
    expect(createdKeyState().value).toBeNull();
    expect(form.busy.value).toBe(false);
  });
});
