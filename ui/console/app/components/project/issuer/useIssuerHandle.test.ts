import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ProjectSchema } from '#gen/project/v1/project_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { defaultHandle, handleProblem, isHandleFormat, slugify, useIssuerHandle } from './useIssuerHandle';

const orgId = 'V3StGXR8Z5jdHj6BmyT2a';
const projectId = 'x6MH9WPWjktMwCBKHy4pF';

describe('isHandleFormat', () => {
  it('takes 3 to 40 lowercase letters, digits and inner hyphens', () => {
    expect(isHandleFormat('acme-web')).toBe(true);
    expect(isHandleFormat('a1b')).toBe(true);
    expect(isHandleFormat('a'.repeat(40))).toBe(true);
    expect(isHandleFormat('ab')).toBe(false);
    expect(isHandleFormat('a'.repeat(41))).toBe(false);
    expect(isHandleFormat('-acme')).toBe(false);
    expect(isHandleFormat('acme-')).toBe(false);
    expect(isHandleFormat('Acme')).toBe(false);
    expect(isHandleFormat('ac me')).toBe(false);
  });
});

describe('defaultHandle', () => {
  it('joins the org and project slugs, cut to 40 without a trailing hyphen', () => {
    expect(defaultHandle('acme', 'web')).toBe('acme-web');
    expect(defaultHandle('a'.repeat(39), 'web')).toBe('a'.repeat(39));
    expect(defaultHandle('acme', '')).toBe('');
  });
});

describe('handleProblem', () => {
  it('says whether a handle breaks the length or the format rule', () => {
    expect(handleProblem('acme-web')).toBe('');
    expect(handleProblem('ab')).toBe('length');
    expect(handleProblem('a'.repeat(41))).toBe('length');
    expect(handleProblem('Partners!')).toBe('format');
    expect(handleProblem('-acme')).toBe('format');
  });
});

describe('slugify', () => {
  it('turns a project name into lowercase words joined by hyphens', () => {
    expect(slugify('Test Shop')).toBe('test-shop');
    expect(slugify('  Café -- Ünïcode! ')).toBe('cafe-unicode');
    expect(slugify('日本')).toBe('');
  });
});

describe('useIssuerHandle', () => {
  let checkIssuerHandle: ReturnType<typeof vi.fn>;
  let updateProjectIssuer: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.useFakeTimers();
    checkIssuerHandle = vi.fn().mockResolvedValue({ available: true, reason: 'ok' });
    updateProjectIssuer = vi.fn(async ({ issuerHandle }: { issuerHandle: string }) => ({
      project: create(ProjectSchema, { id: projectId, issuerHandle, issuerUrl: `https://auth.test/p/${issuerHandle}` }),
    }));
    setClient('$projectClient', { checkIssuerHandle, updateProjectIssuer });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('stays idle on the saved handle and checks a new one after 400 ms of quiet', async () => {
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web' });
    expect(h.status.value).toBe('idle');
    h.input('acme-w');
    h.input('acme-wb');
    h.input('acme-www');
    expect(h.status.value).toBe('checking');
    await vi.advanceTimersByTimeAsync(399);
    expect(checkIssuerHandle).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(checkIssuerHandle).toHaveBeenCalledTimes(1);
    expect(checkIssuerHandle).toHaveBeenCalledWith({ orgId, projectId, issuerHandle: 'acme-www' });
    expect(h.status.value).toBe('available');
    expect(h.canSave.value).toBe(true);
    h.input('acme-web');
    expect(h.status.value).toBe('idle');
    expect(h.canSave.value).toBe(false);
  });

  it('refuses a bad format without asking the server', async () => {
    const h = useIssuerHandle({ orgId, initial: '' });
    h.input('Acme');
    expect(h.status.value).toBe('invalid');
    await vi.runAllTimersAsync();
    expect(checkIssuerHandle).not.toHaveBeenCalled();
  });

  it.each([
    ['taken', 'taken'],
    ['reserved', 'reserved'],
    ['locked', 'locked'],
    ['invalid', 'invalid'],
    ['something-new', 'invalid'],
  ])('shows the server reason %s as %s', async (reason, status) => {
    checkIssuerHandle.mockResolvedValue({ available: false, reason });
    const h = useIssuerHandle({ orgId, initial: '' });
    h.input('acme-web');
    await vi.runAllTimersAsync();
    expect(h.status.value).toBe(status);
  });

  it('drops a stale answer and reports a failed check as an error', async () => {
    const first = deferred<{ available: boolean, reason: string }>();
    checkIssuerHandle.mockReturnValueOnce(first.promise).mockRejectedValueOnce(new Error('down'));
    const h = useIssuerHandle({ orgId, initial: '' });
    h.input('acme-one');
    await vi.advanceTimersByTimeAsync(400);
    h.input('acme-two');
    await vi.advanceTimersByTimeAsync(400);
    first.resolve({ available: true, reason: 'ok' });
    await vi.runAllTimersAsync();
    expect(h.status.value).toBe('error');
  });

  it('is locked and never checks or saves when the project handle is locked', async () => {
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web', locked: true });
    expect(h.status.value).toBe('locked');
    h.input('acme-new');
    await vi.runAllTimersAsync();
    expect(checkIssuerHandle).not.toHaveBeenCalled();
    expect(h.status.value).toBe('locked');
    expect(h.canSave.value).toBe(false);
  });

  it('follows the slug until the person edits the handle', () => {
    const h = useIssuerHandle({ orgId, initial: '' });
    h.follow('acme', 'web');
    expect(h.value.value).toBe('acme-web');
    h.follow('acme', 'shop');
    expect(h.value.value).toBe('acme-shop');
    h.input('my-handle');
    h.follow('acme', 'other');
    expect(h.value.value).toBe('my-handle');
  });

  it('asks before saving, then saves, updates the project and settles on the new handle', async () => {
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web' });
    h.input('acme-shop');
    await vi.runAllTimersAsync();
    h.ask();
    expect(h.confirming.value).toBe(true);
    h.cancel();
    expect(h.confirming.value).toBe(false);
    h.ask();
    await h.save();
    expect(updateProjectIssuer).toHaveBeenCalledWith({ id: projectId, issuerHandle: 'acme-shop' });
    expect(useProjectDetailStore().get(projectId)?.issuerUrl).toBe('https://auth.test/p/acme-shop');
    expect(h.confirming.value).toBe(false);
    expect(h.status.value).toBe('idle');
    expect(h.saved.value).toBe(true);
  });

  it('does not ask or save unless the new handle is available', async () => {
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web' });
    h.ask();
    expect(h.confirming.value).toBe(false);
    await h.save();
    expect(updateProjectIssuer).not.toHaveBeenCalled();
  });

  it('places a refused save under the handle field, and other failures on the form', async () => {
    setKnownKeys('apperror.PRJ005', 'apperror.GEN900');
    updateProjectIssuer.mockRejectedValueOnce(connectError('PRJ005', { issuer_handle: 'acme-shop' })).mockRejectedValueOnce(connectError('GEN900'));
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web' });
    h.input('acme-shop');
    await vi.runAllTimersAsync();
    h.ask();
    await h.save();
    expect(h.fieldError.value).toBe('apperror.PRJ005|{"issuer_handle":"acme-shop"}');
    expect(h.error.value).toBeNull();
    h.ask();
    await h.save();
    expect(h.error.value).toEqual({ message: 'apperror.GEN900', reference: 'error.ref|{"id":"req-1"}' });
    expect(h.busy.value).toBe(false);
  });

  it('starts over from a new saved handle', () => {
    const h = useIssuerHandle({ orgId, projectId, initial: 'acme-web' });
    h.input('Acme');
    h.reset('acme-shop');
    expect(h.value.value).toBe('acme-shop');
    expect(h.status.value).toBe('idle');
  });
});
