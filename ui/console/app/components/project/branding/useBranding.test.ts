import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useProjectDetailStore } from '~/stores/projectDetail';
import { ProjectSchema } from '../../../../gen/project/v1/project_pb';
import { connectError, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { useBranding } from './useBranding';

const id = 'x6MH9WPWjktMwCBKHy4pF';
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
  return { type, size, arrayBuffer: async () => new Uint8Array([1, 2, 3]).buffer } as unknown as File;
}

describe('useBranding', () => {
  let getProject: ReturnType<typeof vi.fn>;
  let updateProjectBranding: ReturnType<typeof vi.fn>;
  let uploadProjectLogo: ReturnType<typeof vi.fn>;
  let deleteProjectLogo: ReturnType<typeof vi.fn>;
  let getProjectLogo: ReturnType<typeof vi.fn>;
  let urls: string[];
  let revoked: string[];

  beforeEach(() => {
    setActivePinia(createPinia());
    urls = [];
    revoked = [];
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => {
        const u = `blob:logo-${urls.length + 1}`;
        urls.push(u);
        return u;
      }),
      revokeObjectURL: vi.fn((u: string) => revoked.push(u)),
    });
    getProject = vi.fn().mockResolvedValue({ project: base });
    updateProjectBranding = vi.fn(async (req: { displayName: string, primaryColor: string, supportEmail: string }) => ({
      project: create(ProjectSchema, { ...base, ...req }),
    }));
    uploadProjectLogo = vi.fn().mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: true }) });
    deleteProjectLogo = vi.fn().mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: false }) });
    getProjectLogo = vi.fn().mockResolvedValue({ contentType: 'image/png', content: new Uint8Array([1]) });
    setClient('$projectClient', { getProject, updateProjectBranding, uploadProjectLogo, deleteProjectLogo, getProjectLogo });
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
    expect(b.contrast.value).toEqual({ ratio: '6.3', low: false });
    expect(getProjectLogo).not.toHaveBeenCalled();
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
    expect(b.contrast.value).toEqual({ ratio: '4.5', low: true });
  });

  it('shows a stored logo through a blob URL', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: true }) });
    const b = useBranding(id);
    await b.load();
    expect(getProjectLogo).toHaveBeenCalledWith({ id });
    expect(b.preview.value.logoUrl).toBe('blob:logo-1');
  });

  it('refuses a wrong type or an oversized file before uploading', async () => {
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/svg+xml', 100));
    expect(b.logoError.value).toBe('apperror.PRJ008_type');
    await b.pickLogo(file('image/png', 262_145));
    expect(b.logoError.value).toBe('apperror.PRJ008_size|{"max":256}');
    expect(uploadProjectLogo).not.toHaveBeenCalled();
  });

  it('uploads a logo, then shows it, replacing the old blob URL', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: true }) });
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/webp', 262_144));
    expect(uploadProjectLogo).toHaveBeenCalledWith({ id, content: new Uint8Array([1, 2, 3]) });
    expect(useProjectDetailStore().get(id)?.hasLogo).toBe(true);
    expect(b.preview.value.logoUrl).toBe('blob:logo-2');
    expect(revoked).toEqual(['blob:logo-1']);
    expect(b.logoError.value).toBe('');
  });

  it('removes the logo', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: true }) });
    const b = useBranding(id);
    await b.load();
    await b.removeLogo();
    expect(deleteProjectLogo).toHaveBeenCalledWith({ id });
    expect(b.preview.value.logoUrl).toBe('');
    expect(revoked).toEqual(['blob:logo-1']);
  });

  it('places a refused upload under the logo field', async () => {
    setKnownKeys('apperror.PRJ008');
    uploadProjectLogo.mockRejectedValueOnce(connectError('PRJ008', { reason: 'type' }));
    const b = useBranding(id);
    await b.load();
    await b.pickLogo(file('image/png', 10));
    expect(b.logoError.value).toBeTruthy();
    expect(b.busy.value).toBe(false);
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

  it('resets the form to the defaults and releases the blob URL on dispose', async () => {
    getProject.mockResolvedValue({ project: create(ProjectSchema, { ...base, hasLogo: true }) });
    const b = useBranding(id);
    await b.load();
    b.resetDefaults();
    expect([b.displayName.value, b.primaryColor.value, b.supportEmail.value]).toEqual(['', '', '']);
    b.dispose();
    expect(revoked).toEqual(['blob:logo-1']);
  });
});
