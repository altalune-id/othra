import { create } from '@bufbuild/protobuf';
import { timestampDate, timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useProjectDetailStore } from '~/stores/projectDetail';
import { ProjectSchema } from '../../../../gen/project/v1/project_pb';
import { connectError, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { useLegal } from './useLegal';

const id = 'x6MH9WPWjktMwCBKHy4pF';
const published = new Date('2026-10-12T00:00:00Z');
const withTerms = create(ProjectSchema, {
  id,
  name: 'Acme App',
  termsUrl: 'https://acme.test/terms',
  privacyUrl: 'https://acme.test/privacy',
  termsVersion: 3,
  termsPublishedAt: timestampFromDate(published),
});

describe('useLegal', () => {
  let getProject: ReturnType<typeof vi.fn>;
  let updateProjectTerms: ReturnType<typeof vi.fn>;
  let bumpTermsVersion: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    getProject = vi.fn().mockResolvedValue({ project: withTerms });
    updateProjectTerms = vi.fn(async (req: { termsUrl: string, privacyUrl: string }) => ({
      project: create(ProjectSchema, { ...withTerms, termsUrl: req.termsUrl, privacyUrl: req.privacyUrl }),
    }));
    bumpTermsVersion = vi.fn().mockResolvedValue({ version: 4, publishedAt: timestampFromDate(new Date('2026-10-16T00:00:00Z')) });
    setClient('$projectClient', { getProject, updateProjectTerms, bumpTermsVersion });
  });

  it('loads the links and the published version', async () => {
    const legal = useLegal(id);
    await legal.load();
    expect(legal.termsUrl.value).toBe('https://acme.test/terms');
    expect(legal.privacyUrl.value).toBe('https://acme.test/privacy');
    expect(legal.hasTerms.value).toBe(true);
    expect(legal.version.value).toBe(3);
    expect(legal.publishedAt.value).toEqual(published);
    expect(legal.urlChangedHint.value).toBe(false);
  });

  it('shows nothing about versions until a terms link exists', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { id, name: 'Acme App' }) });
    const legal = useLegal(id);
    await legal.load();
    expect(legal.hasTerms.value).toBe(false);
    expect(legal.publishedAt.value).toBeUndefined();
    legal.askBump();
    expect(legal.bumping.value).toBe(false);
  });

  it('hints that a changed link does not ask anyone to accept again', async () => {
    const legal = useLegal(id);
    await legal.load();
    legal.termsUrl.value = 'https://acme.test/terms-v2';
    expect(legal.urlChangedHint.value).toBe(true);
    legal.termsUrl.value = '  https://acme.test/terms  ';
    expect(legal.urlChangedHint.value).toBe(false);
  });

  it('saves trimmed links and takes the project the server returns', async () => {
    const legal = useLegal(id);
    await legal.load();
    legal.termsUrl.value = ' https://acme.test/terms-v2 ';
    legal.privacyUrl.value = '';
    await legal.save();
    expect(updateProjectTerms).toHaveBeenCalledWith({ id, termsUrl: 'https://acme.test/terms-v2', privacyUrl: '' });
    expect(useProjectDetailStore().get(id)?.termsUrl).toBe('https://acme.test/terms-v2');
    expect(legal.saved.value).toBe(true);
    expect(legal.urlChangedHint.value).toBe(false);
  });

  it('asks before clearing the terms link, and saves only when confirmed', async () => {
    const legal = useLegal(id);
    await legal.load();
    legal.termsUrl.value = '';
    await legal.save();
    expect(legal.clearing.value).toBe(true);
    expect(updateProjectTerms).not.toHaveBeenCalled();
    legal.cancelClear();
    expect(legal.clearing.value).toBe(false);
    await legal.save();
    await legal.confirmClear();
    expect(updateProjectTerms).toHaveBeenCalledWith({ id, termsUrl: '', privacyUrl: 'https://acme.test/privacy' });
    expect(legal.clearing.value).toBe(false);
  });

  it('checks the links against the proto rules before sending', async () => {
    const legal = useLegal(id);
    await legal.load();
    legal.termsUrl.value = 'http://acme.test/terms';
    await legal.save();
    expect(updateProjectTerms).not.toHaveBeenCalled();
    expect(legal.fieldErrors.value.terms_url).toBeTruthy();
  });

  it('shows save errors', async () => {
    setKnownKeys('apperror.GEN900');
    updateProjectTerms.mockRejectedValueOnce(connectError('GEN900'));
    const legal = useLegal(id);
    await legal.load();
    legal.privacyUrl.value = 'https://acme.test/privacy-v2';
    await legal.save();
    expect(legal.error.value).toEqual({ message: 'apperror.GEN900', reference: 'error.ref|{"id":"req-1"}' });
    expect(legal.busy.value).toBe(false);
  });

  it('publishes the next version after confirmation', async () => {
    const legal = useLegal(id);
    await legal.load();
    expect(legal.nextVersion.value).toBe(4);
    legal.askBump();
    expect(legal.bumping.value).toBe(true);
    legal.cancelBump();
    expect(legal.bumping.value).toBe(false);
    legal.askBump();
    await legal.confirmBump();
    expect(bumpTermsVersion).toHaveBeenCalledWith({ id });
    expect(legal.version.value).toBe(4);
    expect(timestampDate(useProjectDetailStore().get(id)!.termsPublishedAt!)).toEqual(new Date('2026-10-16T00:00:00Z'));
    expect(legal.bumpedTo.value).toBe(4);
    expect(legal.bumping.value).toBe(false);
  });

  it('shows a refused bump', async () => {
    setKnownKeys('apperror.PRJ009');
    bumpTermsVersion.mockRejectedValueOnce(connectError('PRJ009')).mockRejectedValueOnce(new Error('boom'));
    const legal = useLegal(id);
    await legal.load();
    legal.askBump();
    await legal.confirmBump();
    expect(legal.error.value).toEqual({ message: 'apperror.PRJ009', reference: 'error.ref|{"id":"req-1"}' });
    legal.askBump();
    await legal.confirmBump();
    expect(legal.error.value).toEqual({ message: 'error.bump_terms' });
    expect(legal.bumpedTo.value).toBe(0);
    expect(legal.version.value).toBe(3);
  });
});
