import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';
import { connectError, deferred, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { useProjectCreate } from './useProjectCreate';

const project = { id: 'x6MH9WPWjktMwCBKHy4pF', orgId: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'web', name: 'Web' };

describe('useProjectCreate', () => {
  let createProject: ReturnType<typeof vi.fn>;
  let checkIssuerHandle: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.useFakeTimers();
    createProject = vi.fn().mockResolvedValue({ project });
    checkIssuerHandle = vi.fn().mockResolvedValue({ available: true, reason: 'ok' });
    setClient('$projectClient', { createProject, checkIssuerHandle });
    useOrgsStore().items = [{ id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme' }];
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('creates a project, adds it and closes', async () => {
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    expect(form.open.value).toBe(true);
    form.name.value = ' Web ';
    form.setSlug(' web ');
    await vi.runAllTimersAsync();
    await form.submit();
    expect(createProject).toHaveBeenCalledWith({ orgId: 'V3StGXR8Z5jdHj6BmyT2a', name: 'Web', slug: 'web', issuerHandle: 'acme-web' });
    expect(useProjectsStore().items).toEqual([project]);
    expect(form.open.value).toBe(false);
    expect([form.name.value, form.slug.value]).toEqual(['', '']);
  });

  it('stays open when the response has no project', async () => {
    createProject.mockResolvedValue({});
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Web';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(form.open.value).toBe(true);
    expect(useProjectsStore().items).toEqual([]);
  });

  it('does not submit a blank name or while busy', async () => {
    const pending = deferred<{ project: typeof project }>();
    createProject.mockReturnValue(pending.promise);
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = '  ';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(createProject).not.toHaveBeenCalled();
    form.name.value = 'Web';
    await vi.runAllTimersAsync();
    const first = form.submit();
    await form.submit();
    form.hide();
    expect(form.open.value).toBe(true);
    pending.resolve({ project });
    await first;
    expect(createProject).toHaveBeenCalledTimes(1);
  });

  it('shows a Connect error with its reference, then the fallback for a plain error', async () => {
    setKnownKeys('apperror.GEN007');
    createProject.mockRejectedValueOnce(connectError('GEN007')).mockRejectedValueOnce(new Error('boom'));
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Web';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(form.error.value).toEqual({ message: 'apperror.GEN007', reference: 'error.ref|{"id":"req-1"}' });
    await vi.runAllTimersAsync();
    await form.submit();
    expect(form.error.value).toEqual({ message: 'error.create_project' });
    form.hide();
    expect(form.open.value).toBe(false);
    expect(form.error.value).toBeNull();
  });

  it('shows a taken slug under the slug input and never sends a request its rules refuse', async () => {
    setKnownKeys('apperror.PRJ002');
    createProject.mockRejectedValueOnce(connectError('PRJ002'));
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Web';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(form.fieldErrors.value).toEqual({ slug: 'apperror.PRJ002' });
    expect(form.error.value).toBeNull();
    form.name.value = ' ';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(createProject).toHaveBeenCalledTimes(1);
    expect(form.fieldErrors.value).toEqual({ name: 'validation.required' });
  });

  it('prefills the issuer handle from the org and the slug until the person edits it, and sends it', async () => {
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Shop';
    form.setSlug('shop');
    expect(form.handle.value.value).toBe('acme-shop');
    await vi.runAllTimersAsync();
    expect(form.handle.status.value).toBe('available');
    form.handle.input('acme-store');
    form.setSlug('shop2');
    expect(form.handle.value.value).toBe('acme-store');
    await vi.runAllTimersAsync();
    await form.submit();
    expect(createProject).toHaveBeenCalledWith({ orgId: 'V3StGXR8Z5jdHj6BmyT2a', name: 'Shop', slug: 'shop2', issuerHandle: 'acme-store' });
    expect(form.handle.value.value).toBe('');
  });

  it('prefills the handle live from the name while the console URL name is empty, which stays random', async () => {
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Test';
    expect(form.handle.value.value).toBe('acme-test');
    form.name.value = 'Test Shop';
    expect(form.handle.value.value).toBe('acme-test-shop');
    form.setSlug('store');
    expect(form.handle.value.value).toBe('acme-store');
    form.setSlug('');
    expect(form.handle.value.value).toBe('acme-test-shop');
    await vi.runAllTimersAsync();
    await form.submit();
    expect(createProject).toHaveBeenCalledWith({ orgId: 'V3StGXR8Z5jdHj6BmyT2a', name: 'Test Shop', slug: '', issuerHandle: 'acme-test-shop' });
  });

  it('keeps Create off while the handle is empty or still being checked', async () => {
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = '日本';
    expect(form.handle.value.value).toBe('');
    expect(form.canSubmit.value).toBe(false);
    await form.submit();
    form.name.value = 'Shop';
    expect(form.handle.status.value).toBe('checking');
    expect(form.canSubmit.value).toBe(false);
    await form.submit();
    expect(createProject).not.toHaveBeenCalled();
    await vi.runAllTimersAsync();
    expect(form.canSubmit.value).toBe(true);
  });

  it('does not create with a handle the check refused', async () => {
    checkIssuerHandle.mockResolvedValue({ available: false, reason: 'taken' });
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Shop';
    form.setSlug('shop');
    await vi.runAllTimersAsync();
    expect(form.canSubmit.value).toBe(false);
    await form.submit();
    expect(createProject).not.toHaveBeenCalled();
  });

  it('shows a taken handle from the server under the handle input', async () => {
    setKnownKeys('apperror.PRJ005');
    createProject.mockRejectedValueOnce(connectError('PRJ005', { issuer_handle: 'acme-web' }));
    const form = useProjectCreate('V3StGXR8Z5jdHj6BmyT2a');
    form.show();
    form.name.value = 'Web';
    await vi.runAllTimersAsync();
    await form.submit();
    expect(form.fieldErrors.value).toEqual({ issuer_handle: 'apperror.PRJ005|{"issuer_handle":"acme-web"}' });
  });
});
