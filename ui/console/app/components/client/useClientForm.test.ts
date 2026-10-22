import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AuthMethod, ClientKind, ClientSchema, ConsentPolicy } from '../../../gen/client/v1/client_pb';
import { connectError, deferred, setClient, setKnownKeys } from '../../../test/nuxt-globals';
import { useClientForm } from './useClientForm';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const clientId = 'dPm7fmQRJKPVw8PxrgdrN';
const web = create(ClientSchema, { id: clientId, name: 'Store web', kind: ClientKind.WEB, redirectUris: ['https://store.example/cb'], refreshTokens: true, consentPolicy: ConsentPolicy.REQUIRE, pkceRequired: true });
const cli = create(ClientSchema, { id: 'H9MspUj32ux8pfYKYsu1a', name: 'Store CLI', kind: ClientKind.CLI, redirectUris: ['http://127.0.0.1/callback'] });

describe('useClientForm (create)', () => {
  let createClient: ReturnType<typeof vi.fn>;
  let generateClientSecret: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    createClient = vi.fn().mockResolvedValue({ client: web });
    generateClientSecret = vi.fn().mockResolvedValue({ client: web, secret: 'othra_cs_abc' });
    setClient('$clientClient', { createClient, generateClientSecret });
  });

  it('creates a web app, then generates its first secret for the dialog', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = ' Store web ';
    f.fields.value.redirectUris = ['https://store.example/cb', ''];
    f.fields.value.postLogoutUris = ['https://store.example/'];
    f.fields.value.consent = 'implicit';
    const saved = await f.submit();
    expect(createClient).toHaveBeenCalledWith({
      projectId,
      name: 'Store web',
      kind: ClientKind.WEB,
      redirectUris: ['https://store.example/cb'],
      postLogoutRedirectUris: ['https://store.example/'],
      refreshTokens: false,
      consentPolicy: ConsentPolicy.IMPLICIT,
      authMethod: AuthMethod.CLIENT_SECRET_BASIC,
      pkceRequired: true,
    });
    expect(generateClientSecret).toHaveBeenCalledWith({ projectId, clientId });
    expect(saved?.id).toBe(clientId);
    expect(f.revealed.value).toEqual({ clientId, secret: 'othra_cs_abc' });
    expect(f.done.value).toBe(true);
    f.forget();
    expect(f.revealed.value).toBeNull();
  });

  it('creates a CLI app with no secret and no sign-out URIs', async () => {
    createClient.mockResolvedValue({ client: cli });
    const f = useClientForm(projectId);
    f.fields.value.postLogoutUris = ['https://store.example/'];
    f.setKind('cli');
    expect(f.kind.value.secret).toBe(false);
    expect(f.fields.value.postLogoutUris).toEqual([]);
    f.fields.value.name = 'Store CLI';
    f.fields.value.redirectUris = ['http://127.0.0.1/callback'];
    await f.submit();
    expect(createClient.mock.calls[0]![0]).toMatchObject({ kind: ClientKind.CLI, postLogoutRedirectUris: [], authMethod: AuthMethod.UNSPECIFIED });
    expect(createClient.mock.calls[0]![0]).not.toHaveProperty('pkceRequired', false);
    expect(createClient.mock.calls[0]![0].pkceRequired).toBeUndefined();
    expect(generateClientSecret).not.toHaveBeenCalled();
    expect(f.revealed.value).toBeNull();
  });

  it('keeps the client when its first secret fails, and says so', async () => {
    setKnownKeys('apperror.CLT005');
    generateClientSecret.mockRejectedValue(connectError('CLT005'));
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://store.example/cb'];
    expect((await f.submit())?.id).toBe(clientId);
    expect(f.secretError.value?.message).toBe('apperror.CLT005');
    expect(f.revealed.value).toBeNull();
  });

  it('refuses to send rows that break the rules, with a message under each', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = '';
    f.fields.value.redirectUris = ['https://app.example/cb#x', 'http://localhost:5173/cb', 'https://Store.Example/cb'];
    f.fields.value.postLogoutUris = ['https://*.example/'];
    expect(await f.submit()).toBeNull();
    expect(createClient).not.toHaveBeenCalled();
    expect(f.uriErrors.value.redirect).toEqual(['apperror.CLT002_fragment', '', 'apperror.CLT002_not_canonical|{"canonical":"https://store.example/cb"}']);
    expect(f.uriErrors.value.postLogout).toEqual(['apperror.CLT002_wildcard']);
    expect(f.nameError.value).toBe('validation.required');
    expect(f.warnings.value.redirect).toEqual([false, true, false]);
    expect(f.warnings.value.postLogout).toEqual([false]);
  });

  it('flags the first row when no redirect URI is given', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    expect(await f.submit()).toBeNull();
    expect(f.uriErrors.value.redirect).toEqual(['apperror.CLT002_required']);
  });

  it('drops a row\'s message while it is edited and adds none, so nothing moves under the next click', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://a.example/cb#x', 'nope'];
    f.fields.value.postLogoutUris = ['nope'];
    await f.submit();
    expect(f.uriErrors.value.redirect).toEqual(['apperror.CLT002_fragment', 'apperror.CLT002_unparsable']);
    f.fields.value.redirectUris = ['https://a.example/cb#y', 'nope'];
    f.clearRow('redirect', 0);
    expect(f.uriErrors.value.redirect).toEqual(['', 'apperror.CLT002_unparsable']);
    f.clearRow('postLogout', 0);
    expect(f.uriErrors.value.postLogout).toEqual(['']);
    f.clearRow('redirect', 5);
    expect(f.uriErrors.value.redirect).toEqual(['', 'apperror.CLT002_unparsable']);
  });

  it('removes a row and its message, keeping one redirect row', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://a.example/cb#x', 'https://b.example/cb'];
    await f.submit();
    expect(f.uriErrors.value.redirect).toEqual(['apperror.CLT002_fragment', '']);
    f.removeRow('redirect', 0);
    expect(f.fields.value.redirectUris).toEqual(['https://b.example/cb']);
    expect(f.uriErrors.value.redirect).toEqual(['']);
    f.removeRow('redirect', 0);
    expect(f.fields.value.redirectUris).toEqual(['']);
    f.fields.value.postLogoutUris = ['https://a.example/'];
    f.removeRow('postLogout', 0);
    expect(f.fields.value.postLogoutUris).toEqual([]);
  });

  it('puts a server refusal under the row it names, by index or by URI', async () => {
    setKnownKeys('apperror.CLT002_not_canonical', 'apperror.CLT002_duplicate');
    createClient.mockRejectedValueOnce(connectError('CLT002', { field: 'redirect_uris[1]', reason: 'not_canonical', canonical: 'https://b.example/cb' }));
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://a.example/cb', '', 'https://b.example/cb'];
    f.fields.value.postLogoutUris = ['https://a.example/'];
    await f.submit();
    expect(f.uriErrors.value.redirect[2]).toMatch(/^apperror\.CLT002_not_canonical\|.*"canonical":"https:\/\/b\.example\/cb"/);
    expect(f.error.value).toBeNull();

    createClient.mockRejectedValueOnce(connectError('CLT002', { field: 'post_logout_redirect_uris', uri: 'https://a.example/', reason: 'duplicate' }));
    await f.submit();
    expect(f.uriErrors.value.postLogout[0]).toMatch(/^apperror\.CLT002_duplicate\|/);
  });

  it('shows a refusal it cannot place, and the name rule under the name', async () => {
    setKnownKeys('apperror.CLT002', 'apperror.GEN900');
    createClient.mockRejectedValueOnce(connectError('CLT002', { field: 'redirect_uris', uri: 'https://zzz.example/' }));
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://a.example/cb'];
    await f.submit();
    expect(f.error.value?.message).toMatch(/^apperror\.CLT002\|/);
    createClient.mockRejectedValueOnce(connectError('GEN900'));
    await f.submit();
    expect(f.error.value?.message).toBe('apperror.GEN900');
  });

  it('ignores a second submit while one runs, and a kind change on a saved client', async () => {
    const pending = deferred<{ client: typeof web }>();
    createClient.mockReturnValue(pending.promise);
    const f = useClientForm(projectId);
    f.fields.value.name = 'Store web';
    f.fields.value.redirectUris = ['https://a.example/cb'];
    const first = f.submit();
    expect(await f.submit()).toBeNull();
    pending.resolve({ client: web });
    await first;
    expect(createClient).toHaveBeenCalledTimes(1);
  });

  it('flags a name over 100 characters', async () => {
    const f = useClientForm(projectId);
    f.fields.value.name = 'x'.repeat(101);
    f.fields.value.redirectUris = ['https://a.example/cb'];
    await f.submit();
    expect(f.nameError.value).toMatch(/^validation\./);
    expect(createClient).not.toHaveBeenCalled();
  });
});

describe('useClientForm (edit)', () => {
  let updateClient: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    updateClient = vi.fn().mockResolvedValue({ client: { ...web, name: 'Renamed' } });
    setClient('$clientClient', { updateClient });
  });

  it('starts from the saved client, keeps its kind, and saves the fields', async () => {
    const f = useClientForm(projectId, web);
    expect(f.fields.value.name).toBe('Store web');
    f.setKind('cli');
    expect(f.fields.value.kind).toBe('web');
    f.fields.value.name = 'Renamed';
    const saved = await f.submit();
    expect(updateClient).toHaveBeenCalledWith({
      projectId,
      clientId,
      name: 'Renamed',
      redirectUris: ['https://store.example/cb'],
      postLogoutRedirectUris: [],
      refreshTokens: true,
      consentPolicy: ConsentPolicy.REQUIRE,
      authMethod: AuthMethod.CLIENT_SECRET_BASIC,
      pkceRequired: true,
    });
    expect(saved?.name).toBe('Renamed');
    expect(f.done.value).toBe(true);
    expect(f.revealed.value).toBeNull();
  });

  it('turns a server violation on the name into its message', async () => {
    const f = useClientForm(projectId, web);
    f.fields.value.name = '';
    await f.submit();
    expect(f.nameError.value).toBe('validation.required');
    expect(updateClient).not.toHaveBeenCalled();
  });
});

describe('useClientForm (M5 web settings)', () => {
  it('sends the post method and PKCE off for a web app, and shows the server refusal', async () => {
    setKnownKeys('apperror.CLT003_pkce_native');
    const updateClient = vi.fn().mockRejectedValueOnce(connectError('CLT003', { reason: 'pkce_native' })).mockResolvedValue({ client: web });
    setClient('$clientClient', { updateClient });
    const f = useClientForm(projectId, web);
    f.fields.value.authMethod = 'post';
    f.fields.value.pkceRequired = false;
    await f.submit();
    expect(updateClient.mock.calls[0]![0]).toMatchObject({ authMethod: AuthMethod.CLIENT_SECRET_POST, pkceRequired: false });
    expect(f.error.value?.message).toMatch(/^apperror\.CLT003_pkce_native/);
  });
});
