import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ProjectSchema } from '#gen/project/v1/project_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useProjectDetailStore } from '~/stores/projectDetail';
import { putHeaders, useBranding } from './useBranding';

const id = 'x6MH9WPWjktMwCBKHy4pF';
const logoUrl = 'https://files.altalune.id/public/othra/projects/x6MH9WPWjktMwCBKHy4pF/logo/fil_new.webp';
const base = create(ProjectSchema, {
  id,
  name: 'Acme App',
  displayName: 'Acme',
  primaryColor: '#4f46e5',
  supportEmail: 'help@acme.test',
  termsUrl: 'https://acme.test/terms',
  privacyUrl: '',
});

function file(type: string, size: number) {
  return { type, size } as unknown as File;
}

describe('putHeaders', () => {
  it('keeps the signed headers a browser may set and drops the ones it may not', () => {
    expect(putHeaders({ 'Content-Type': 'image/webp', 'x-amz-meta-a': 'b', 'Content-Length': '10', 'Host': 'files' }))
      .toEqual({ 'Content-Type': 'image/webp', 'x-amz-meta-a': 'b' });
    expect(putHeaders({})).toEqual({});
  });
});

describe('useBranding', () => {
  let getProject: ReturnType<typeof vi.fn>;
  let whoami: ReturnType<typeof vi.fn>;
  let updateProjectBranding: ReturnType<typeof vi.fn>;
  let createProjectLogoUpload: ReturnType<typeof vi.fn>;
  let completeProjectLogoUpload: ReturnType<typeof vi.fn>;
  let deleteProjectLogo: ReturnType<typeof vi.fn>;
  let put: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    getProject = vi.fn().mockResolvedValue({ project: base });
    whoami = vi.fn().mockResolvedValue({ source: 'local', userId: 'u1', fileUploads: true });
    updateProjectBranding = vi.fn(async (req: { displayName: string, primaryColor: string, supportEmail: string }) => ({
      project: create(ProjectSchema, { ...base, ...req }),
    }));
    createProjectLogoUpload = vi.fn().mockResolvedValue({
      uploadId: 'upl_1',
      url: 'http://localhost:3900/altalune-files/public/othra/logo.webp?X-Amz-Signature=abc',
      method: 'PUT',
      headers: { 'content-type': 'image/webp', 'content-length': '262144' },
    });
    completeProjectLogoUpload = vi.fn().mockResolvedValue({ project: create(ProjectSchema, { ...base, logoUrl }) });
    deleteProjectLogo = vi.fn().mockResolvedValue({ project: create(ProjectSchema, { ...base, logoUrl: '' }) });
    put = vi.fn().mockResolvedValue({ ok: true, status: 200 });
    vi.stubGlobal('fetch', put);
    setClient('$authClient', { whoami });
    setClient('$projectClient', { getProject, updateProjectBranding, createProjectLogoUpload, completeProjectLogoUpload, deleteProjectLogo });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('loads the form and builds the preview with the readable button text', async () => {
    const b = useBranding(id);
    await b.load();
    expect([b.displayName.value, b.primaryColor.value, b.supportEmail.value]).toEqual(['Acme', '#4f46e5', 'help@acme.test']);
    expect(b.preview.value).toEqual({
      name: 'Acme',
      logoUrl: '',
      primary: '#4f46e5',
      primaryContent: '#ffffff',
      termsUrl: 'https://acme.test/terms',
      privacyUrl: '',
      supportEmail: 'help@acme.test',
    });
    expect(b.contrast.value).toEqual({ ratio: '6.2', low: false });
    expect(b.storageOff.value).toBe(false);
  });

  it('shows a stored logo straight from its public URL', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, logoUrl }) });
    const b = useBranding(id);
    await b.load();
    expect(b.preview.value.logoUrl).toBe(logoUrl);
  });

  it('falls back to the project name and the theme colour, and warns on low contrast', async () => {
    const b = useBranding(id);
    await b.load();
    b.displayName.value = '  ';
    b.primaryColor.value = '';
    expect(b.preview.value.name).toBe('Acme App');
    expect(b.preview.value.primary).toBe('');
    expect(b.contrast.value).toBeNull();
    b.primaryColor.value = '#777';
    expect(b.preview.value.primary).toBe('#777777');
    expect(b.contrast.value).toEqual({ ratio: '4.4', low: true });
  });

  it('refuses a wrong type or an oversized file before asking for an upload', async () => {
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/svg+xml', 100));
    expect(b.logoError.value).toBe('apperror.PRJ008_type');
    await b.pickLogo(file('image/png', 262_145));
    expect(b.logoError.value).toBe('apperror.PRJ008_size|{"max":256}');
    expect(createProjectLogoUpload).not.toHaveBeenCalled();
    expect(put).not.toHaveBeenCalled();
  });

  it('uploads straight to storage: create, put the file with the signed headers, then complete', async () => {
    const b = useBranding(id);
    await b.load();
    const f = file('image/webp', 262_144);
    const pending = b.pickLogo(f);
    expect(b.uploading.value).toBe(true);
    await pending;
    expect(createProjectLogoUpload).toHaveBeenCalledWith({ id, contentType: 'image/webp', sizeBytes: 262_144 });
    expect(put).toHaveBeenCalledWith('http://localhost:3900/altalune-files/public/othra/logo.webp?X-Amz-Signature=abc', {
      method: 'PUT',
      headers: { 'content-type': 'image/webp' },
      body: f,
    });
    expect(completeProjectLogoUpload).toHaveBeenCalledWith({ id, uploadId: 'upl_1' });
    expect(useProjectDetailStore().get(id)?.logoUrl).toBe(logoUrl);
    expect(b.preview.value.logoUrl).toBe(logoUrl);
    expect(b.logoError.value).toBe('');
    expect(b.uploading.value).toBe(false);
  });

  it('confirms a finished upload until the next logo change', async () => {
    const b = useBranding(id);
    await b.load();
    expect(b.logoSaved.value).toBe(false);
    await b.pickLogo(file('image/png', 10));
    expect(b.logoSaved.value).toBe(true);
    await b.removeLogo();
    expect(b.logoSaved.value).toBe(false);
    await b.pickLogo(file('image/png', 10));
    expect(b.logoSaved.value).toBe(true);
    put.mockResolvedValueOnce({ ok: false, status: 403 });
    await b.pickLogo(file('image/png', 10));
    expect(b.logoSaved.value).toBe(false);
  });

  it('disables uploads when the server has no file storage', async () => {
    whoami.mockResolvedValue({ source: 'local', userId: 'u1', fileUploads: false });
    const b = useBranding(id);
    await b.load();
    expect(b.storageOff.value).toBe(true);
    await b.pickLogo(file('image/png', 10));
    expect(createProjectLogoUpload).not.toHaveBeenCalled();
  });

  it('falls back to STO003 from the server and stops asking', async () => {
    setKnownKeys('apperror.STO003');
    createProjectLogoUpload.mockRejectedValueOnce(connectError('STO003'));
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/png', 10));
    expect(b.storageOff.value).toBe(true);
    expect(b.logoError.value).toBe('');
    expect(put).not.toHaveBeenCalled();
    await b.pickLogo(file('image/png', 10));
    expect(createProjectLogoUpload).toHaveBeenCalledTimes(1);
  });

  it('shows a refused upload request and never puts the file', async () => {
    setKnownKeys('apperror.PRJ008');
    createProjectLogoUpload.mockRejectedValueOnce(connectError('PRJ008', { reason: 'size', max: '256' }));
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/png', 10));
    expect(b.logoError.value).toBeTruthy();
    expect(put).not.toHaveBeenCalled();
    expect(b.uploading.value).toBe(false);
  });

  it('does not complete when storage refuses the file or the request fails', async () => {
    const b = useBranding(id);
    await b.load();
    put.mockResolvedValueOnce({ ok: false, status: 403 });
    await b.pickLogo(file('image/png', 10));
    expect(b.logoError.value).toBe('branding.logo_upload_failed');
    put.mockRejectedValueOnce(new TypeError('Failed to fetch'));
    await b.pickLogo(file('image/png', 10));
    expect(b.logoError.value).toBe('branding.logo_upload_failed');
    expect(completeProjectLogoUpload).not.toHaveBeenCalled();
    expect(b.uploading.value).toBe(false);
  });

  it('shows a refused completion and keeps the old logo', async () => {
    setKnownKeys('apperror.STO002');
    completeProjectLogoUpload.mockRejectedValueOnce(connectError('STO002', { reason: 'type' }));
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/png', 10));
    expect(b.logoError.value).toBeTruthy();
    expect(useProjectDetailStore().get(id)?.logoUrl).toBe('');
  });

  it('keeps the old logo when completion returns no project', async () => {
    completeProjectLogoUpload.mockResolvedValueOnce({});
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/png', 10));
    expect(b.preview.value.logoUrl).toBe('');
    expect(b.logoError.value).toBe('');
  });

  it('removes the logo, and shows a failed removal', async () => {
    setKnownKeys('apperror.GEN900');
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, logoUrl }) });
    const b = useBranding(id);
    await b.load();
    await b.removeLogo();
    expect(deleteProjectLogo).toHaveBeenCalledWith({ id });
    expect(b.preview.value.logoUrl).toBe('');
    deleteProjectLogo.mockRejectedValueOnce(connectError('GEN900'));
    await b.removeLogo();
    expect(b.logoError.value).toBe('apperror.GEN900');
  });

  it('saves trimmed values with a normalised colour', async () => {
    const b = useBranding(id);
    await b.load();
    b.displayName.value = ' Acme Co ';
    b.primaryColor.value = '#ABC';
    b.supportEmail.value = ' ';
    await b.save();
    expect(updateProjectBranding).toHaveBeenCalledWith({ id, displayName: 'Acme Co', primaryColor: '#aabbcc', supportEmail: '' });
    expect(b.saved.value).toBe(true);
  });

  it('clears "Saved" on the next edit and on reset', async () => {
    const b = useBranding(id);
    await b.load();
    await b.save();
    expect(b.saved.value).toBe(true);
    b.supportEmail.value = 'support@acme.test';
    expect(b.saved.value).toBe(false);
    await b.save();
    expect(b.saved.value).toBe(true);
    b.resetDefaults();
    expect(b.saved.value).toBe(false);
  });

  it('checks the proto rules before sending', async () => {
    const b = useBranding(id);
    await b.load();
    b.primaryColor.value = 'blue';
    await b.save();
    expect(updateProjectBranding).not.toHaveBeenCalled();
    expect(b.fieldErrors.value.primary_color).toBeTruthy();
  });

  it('shows save errors', async () => {
    setKnownKeys('apperror.GEN900');
    updateProjectBranding.mockRejectedValueOnce(connectError('GEN900'));
    const b = useBranding(id);
    await b.load();
    await b.save();
    expect(b.error.value).toEqual({ message: 'apperror.GEN900', reference: 'error.ref|{"id":"req-1"}' });
  });

  it('resets the form to the defaults', async () => {
    const b = useBranding(id);
    await b.load();
    b.resetDefaults();
    expect([b.displayName.value, b.primaryColor.value, b.supportEmail.value]).toEqual(['', '', '']);
  });
});
