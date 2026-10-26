import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useProjectDetailStore } from '~/stores/projectDetail';
import { ProjectSchema } from '../../../../gen/project/v1/project_pb';
import { UpstreamKind, UpstreamSchema } from '../../../../gen/upstream/v1/upstream_pb';
import { connectError, deferred, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { googleRedirectUri, useSignIn } from './useSignIn';

const id = 'x6MH9WPWjktMwCBKHy4pF';
const upstreamId = 'dPm7fmQRJKPVw8PxrgdrN';
const project = create(ProjectSchema, { id, name: 'Acme App', issuerUrl: 'https://id.acme.test/p/acme', signinEmailLink: true });
const google = create(UpstreamSchema, { id: upstreamId, kind: UpstreamKind.GOOGLE, clientId: 'c-1.apps.googleusercontent.com', secretSet: true, enabled: true, redirectUri: 'https://id.acme.test/p/acme/federation/google/callback' });

describe('googleRedirectUri', () => {
  it('hangs the Google callback under the issuer', () => {
    expect(googleRedirectUri('https://id.acme.test/p/acme/')).toBe('https://id.acme.test/p/acme/federation/google/callback');
    expect(googleRedirectUri('')).toBe('');
  });
});

describe('useSignIn', () => {
  let projectClient: Record<string, ReturnType<typeof vi.fn>>;
  let upstreamClient: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    setActivePinia(createPinia());
    projectClient = {
      getProject: vi.fn().mockResolvedValue({ project }),
      updateProjectSignIn: vi.fn(async (req: { emailLink: boolean }) => ({ project: create(ProjectSchema, { ...project, signinEmailLink: req.emailLink }) })),
    };
    upstreamClient = {
      listUpstreams: vi.fn().mockResolvedValue({ upstreams: [google] }),
      createUpstream: vi.fn(async (req: { clientId: string }) => ({ upstream: create(UpstreamSchema, { ...google, clientId: req.clientId }) })),
      updateUpstream: vi.fn(async (req: { clientId: string }) => ({ upstream: create(UpstreamSchema, { ...google, clientId: req.clientId }) })),
      setUpstreamEnabled: vi.fn(async (req: { enabled: boolean }) => ({ upstream: create(UpstreamSchema, { ...google, enabled: req.enabled }) })),
      deleteUpstream: vi.fn().mockResolvedValue({}),
    };
    setClient('$projectClient', projectClient);
    setClient('$upstreamClient', upstreamClient);
  });

  it('loads Google with the server redirect URI, and never holds a secret', async () => {
    const s = useSignIn(id);
    await s.load();
    expect(upstreamClient.listUpstreams).toHaveBeenCalledWith({ projectId: id });
    expect(s.google.value?.id).toBe(upstreamId);
    expect(s.clientId.value).toBe('c-1.apps.googleusercontent.com');
    expect(s.secret.value).toBe('');
    expect(s.redirectUri.value).toBe(google.redirectUri);
    expect(s.emailLink.value).toBe(true);
    expect(s.emailLocked.value).toBe(false);
    expect(s.googleLocked.value).toBe(false);
  });

  it('shows the load error', async () => {
    setKnownKeys('apperror.GEN900');
    upstreamClient.listUpstreams!.mockRejectedValueOnce(connectError('GEN900'));
    const s = useSignIn(id);
    await s.load();
    expect(s.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('sets Google up with the redirect URI derived from the issuer, after checking the form', async () => {
    upstreamClient.listUpstreams!.mockResolvedValue({ upstreams: [] });
    const s = useSignIn(id);
    await s.load();
    expect(s.google.value).toBeNull();
    expect(s.redirectUri.value).toBe('https://id.acme.test/p/acme/federation/google/callback');
    expect(s.emailLocked.value).toBe(true);
    s.startSetup();
    expect(s.settingUp.value).toBe(true);
    await s.save();
    expect(upstreamClient.createUpstream).not.toHaveBeenCalled();
    expect(s.fieldErrors.value.client_id).toBeTruthy();
    expect(s.fieldErrors.value.client_secret).toBeTruthy();
    s.clientId.value = ' c-2.apps.googleusercontent.com ';
    s.secret.value = 'shh';
    await s.save();
    expect(upstreamClient.createUpstream).toHaveBeenCalledWith({ projectId: id, kind: UpstreamKind.GOOGLE, clientId: 'c-2.apps.googleusercontent.com', clientSecret: 'shh', enabled: true });
    expect(s.settingUp.value).toBe(false);
    expect(s.saved.value).toBe(true);
    expect(s.secret.value).toBe('');
    s.clientId.value = 'x';
    expect(s.saved.value).toBe(false);
  });

  it('cancels a setup and shows a refused create', async () => {
    setKnownKeys('apperror.UPS003');
    upstreamClient.listUpstreams!.mockResolvedValue({ upstreams: [] });
    upstreamClient.createUpstream!.mockRejectedValueOnce(connectError('UPS003'));
    const s = useSignIn(id);
    await s.load();
    s.startSetup();
    s.clientId.value = 'c';
    s.secret.value = 's';
    await s.save();
    expect(s.fieldErrors.value.client_id).toBe('apperror.UPS003');
    expect(s.settingUp.value).toBe(true);
    s.cancelSetup();
    expect(s.settingUp.value).toBe(false);
    expect(s.fieldErrors.value.client_id).toBeUndefined();
  });

  it('keeps the stored secret unless it is replaced', async () => {
    const s = useSignIn(id);
    await s.load();
    s.clientId.value = 'c-3';
    s.secret.value = 'ignored';
    await s.save();
    expect(upstreamClient.updateUpstream).toHaveBeenLastCalledWith({ projectId: id, upstreamId, clientId: 'c-3', clientSecret: '' });
    s.replacingSecret.value = true;
    s.secret.value = 'new';
    await s.save();
    expect(upstreamClient.updateUpstream).toHaveBeenLastCalledWith({ projectId: id, upstreamId, clientId: 'c-3', clientSecret: 'new' });
    expect(s.replacingSecret.value).toBe(false);
  });

  it('refuses an empty client ID before sending an update, and ignores a save while one runs', async () => {
    const s = useSignIn(id);
    await s.load();
    s.clientId.value = ' ';
    await s.save();
    expect(upstreamClient.updateUpstream).not.toHaveBeenCalled();
    expect(s.fieldErrors.value.client_id).toBeTruthy();
    const pending = deferred<object>();
    upstreamClient.updateUpstream!.mockReturnValueOnce(pending.promise);
    s.clientId.value = 'c';
    const first = s.save();
    await s.save();
    pending.resolve({ upstream: google });
    await first;
    expect(upstreamClient.updateUpstream).toHaveBeenCalledTimes(1);
  });

  it('turns the email link and Google on and off, and locks the last method', async () => {
    const s = useSignIn(id);
    await s.load();
    await s.setEmailLink(false);
    expect(projectClient.updateProjectSignIn).toHaveBeenCalledWith({ id, emailLink: false });
    expect(s.emailLink.value).toBe(false);
    expect(s.googleLocked.value).toBe(true);
    await s.setEmailLink(true);
    await s.setGoogleEnabled(false);
    expect(upstreamClient.setUpstreamEnabled).toHaveBeenCalledWith({ projectId: id, upstreamId, enabled: false });
    expect(s.googleOn.value).toBe(false);
    expect(s.emailLocked.value).toBe(true);
  });

  it('shows a refused toggle and keeps the old state', async () => {
    setKnownKeys('apperror.PRJ010');
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010'));
    upstreamClient.setUpstreamEnabled!.mockRejectedValueOnce(connectError('PRJ010'));
    const s = useSignIn(id);
    await s.load();
    await s.setEmailLink(false);
    expect(s.emailError.value).toBeNull();
    expect(s.emailLockReason.value).toBe('unlinked');
    expect(s.toggleError.value).toBeNull();
    expect(s.emailLink.value).toBe(true);
    await s.setGoogleEnabled(false);
    expect(s.toggleError.value?.message).toBe('apperror.PRJ010');
    expect(s.googleOn.value).toBe(true);
  });

  it('does nothing without a project or a Google upstream', async () => {
    upstreamClient.listUpstreams!.mockResolvedValue({ upstreams: [] });
    const s = useSignIn(id);
    await s.setEmailLink(false);
    expect(projectClient.updateProjectSignIn).not.toHaveBeenCalled();
    await s.load();
    await s.setGoogleEnabled(false);
    s.askRemove();
    await s.confirmRemove();
    expect(upstreamClient.setUpstreamEnabled).not.toHaveBeenCalled();
    expect(upstreamClient.deleteUpstream).not.toHaveBeenCalled();
  });

  it('removes Google after a confirm, and keeps the confirm open on a refusal', async () => {
    setKnownKeys('apperror.UPS004');
    upstreamClient.deleteUpstream!.mockRejectedValueOnce(connectError('UPS004'));
    const s = useSignIn(id);
    await s.load();
    await s.confirmRemove();
    expect(upstreamClient.deleteUpstream).not.toHaveBeenCalled();
    s.askRemove();
    await s.confirmRemove();
    expect(s.removeError.value?.message).toBe('apperror.UPS004');
    expect(s.removing.value).toBe(true);
    s.cancelRemove();
    expect(s.removing.value).toBe(false);
    s.askRemove();
    await s.confirmRemove();
    expect(upstreamClient.deleteUpstream).toHaveBeenCalledWith({ projectId: id, upstreamId });
    expect(s.google.value).toBeNull();
    expect(s.removing.value).toBe(false);
  });

  it('keeps the confirm open while a removal runs', async () => {
    const pending = deferred<object>();
    upstreamClient.deleteUpstream!.mockReturnValue(pending.promise);
    const s = useSignIn(id);
    await s.load();
    s.askRemove();
    const first = s.confirmRemove();
    s.cancelRemove();
    expect(s.removing.value).toBe(true);
    pending.resolve({});
    await first;
    expect(s.removing.value).toBe(false);
  });

  it('marks Google locked for removal once end users linked it', async () => {
    upstreamClient.listUpstreams!.mockResolvedValue({ upstreams: [create(UpstreamSchema, { ...google, identitiesLinked: true })] });
    const s = useSignIn(id);
    await s.load();
    expect(s.removeLocked.value).toBe(true);
    expect(useProjectDetailStore().get(id)?.name).toBe('Acme App');
  });

  it('locks the email link with the reason the server refused it for (A4)', async () => {
    setKnownKeys('apperror.PRJ010');
    const s = useSignIn(id);
    await s.load();
    expect(s.emailLockReason.value).toBe('');
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010'));
    await s.setEmailLink(false);
    expect(s.emailLockReason.value).toBe('unlinked');
    expect(s.emailLocked.value).toBe(true);
    expect(s.emailLink.value).toBe(true);
    await s.setGoogleEnabled(false);
    expect(s.emailLockReason.value).toBe('google_off');
  });

  it('keeps the email link unlocked when the refusal is about Google being off, or not PRJ010', async () => {
    setKnownKeys('apperror.PRJ010', 'apperror.GEN900');
    const s = useSignIn(id);
    await s.load();
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010', { reason: 'google_off' }));
    await s.setEmailLink(false);
    expect(s.emailLockReason.value).toBe('');
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('GEN900'));
    await s.setEmailLink(false);
    expect(s.emailLockReason.value).toBe('');
    expect(s.emailError.value?.message).toBe('apperror.GEN900');
    await s.setEmailLink(false);
    expect(s.emailError.value).toBeNull();
    expect(s.emailLink.value).toBe(false);
  });

  it('shows only the lock line for an unlinked refusal, and clears the email error after any later save (MF3)', async () => {
    setKnownKeys('apperror.PRJ010');
    const s = useSignIn(id);
    await s.load();
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010', { reason: 'google_off' }));
    await s.setEmailLink(false);
    expect(s.emailError.value?.message).toBe('apperror.PRJ010');
    await s.setGoogleEnabled(true);
    expect(s.emailError.value).toBeNull();
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010', { reason: 'google_off' }));
    await s.setEmailLink(false);
    s.clientId.value = 'c-9';
    await s.save();
    expect(s.emailError.value).toBeNull();
    projectClient.updateProjectSignIn!.mockRejectedValueOnce(connectError('PRJ010', { reason: 'google_off' }));
    await s.setEmailLink(false);
    s.askRemove();
    await s.confirmRemove();
    expect(s.emailError.value).toBeNull();
  });
});
