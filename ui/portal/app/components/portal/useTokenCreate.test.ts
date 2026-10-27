import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { tokenView } from '#base/lib/account/views';
import { PersonalTokenSchema, PersonalTokenState, TokenExpiry, TokenServerSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useTokenCreate } from './useTokenCreate';

const server = create(TokenServerSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes MCP', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read'] });
const now = new Date('2026-10-26T00:00:00Z');
const token = create(PersonalTokenSchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'cli', hint: 'ab12', server, scopes: ['notes:read'], state: PersonalTokenState.ACTIVE, expiresAt: timestampFromDate(new Date('2026-10-29T00:00:00Z')) });

describe('useTokenCreate', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    client = {
      listPersonalTokens: vi.fn().mockResolvedValue({ tokens: [token] }),
      revokePersonalToken: vi.fn().mockResolvedValue({}),
      listTokenServers: vi.fn().mockResolvedValue({ servers: [server] }),
      createPersonalToken: vi.fn().mockResolvedValue({ token, secret: 'othra_pt_secret' }),
    };
    setClient('$accountClient', client);
  });

  it('picks the only server, checks the form, creates and forgets the secret', async () => {
    const f = useTokenCreate();
    await f.load();
    expect(f.serverId.value).toBe('dPm7fmQRJKPVw8PxrgdrN');
    expect(await f.submit()).toBe(false);
    expect(f.fieldErrors.value.name).toBeTruthy();
    f.name.value = ' cli ';
    f.scopes.value = ['notes:read'];
    f.expiry.value = TokenExpiry.TOKEN_EXPIRY_7_DAYS;
    expect(await f.submit()).toBe(true);
    expect(client.createPersonalToken).toHaveBeenCalledWith({ name: 'cli', serverId: 'dPm7fmQRJKPVw8PxrgdrN', scopes: ['notes:read'], expiry: TokenExpiry.TOKEN_EXPIRY_7_DAYS });
    expect(f.secret.value).toBe('othra_pt_secret');
    f.forget();
    expect(f.secret.value).toBe('');
  });

  it('asks for a fresh sign-in on ACC009 and clears the picked scopes on a server change', async () => {
    setKnownKeys('apperror.ACC009', 'apperror.GEN900');
    client.listTokenServers!.mockResolvedValue({ servers: [server, create(TokenServerSchema, { id: 'x6MH9WPWjktMwCBKHy4pF', scopes: ['x'] })] });
    const f = useTokenCreate();
    await f.load();
    expect(f.serverId.value).toBe('');
    f.scopes.value = ['stale'];
    f.serverId.value = 'dPm7fmQRJKPVw8PxrgdrN';
    expect(f.scopes.value).toEqual([]);
    f.name.value = 'cli';
    f.scopes.value = ['notes:read'];
    client.createPersonalToken!.mockRejectedValueOnce(connectError('ACC009'));
    expect(await f.submit()).toBe(false);
    expect(f.reauth.value).toBe(true);
    client.listTokenServers!.mockRejectedValueOnce(connectError('GEN900'));
    await f.load();
    expect(f.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('ignores a second create while one runs and keeps an answer without a token row', async () => {
    let finish!: (v: unknown) => void;
    client.createPersonalToken!.mockReturnValueOnce(new Promise((resolve) => {
      finish = resolve;
    }));
    const f = useTokenCreate();
    await f.load();
    f.name.value = 'cli';
    f.scopes.value = ['notes:read'];
    const first = f.submit();
    expect(await f.submit()).toBe(false);
    finish({ secret: 's' });
    expect(await first).toBe(true);
    expect(tokenView(create(PersonalTokenSchema, { state: PersonalTokenState.ACTIVE, lastUsedAt: timestampFromDate(now) }), now).lastUsedAt).toEqual(now);
  });

  it('builds the MCP snippet only once there is a secret for the picked server', async () => {
    const f = useTokenCreate();
    await f.load();
    expect(f.snippet.value).toBe('');
    f.name.value = 'cli';
    f.scopes.value = ['notes:read'];
    await f.submit();
    expect(f.snippet.value).toContain('https://mcp.acme.test/mcp');
    expect(f.snippet.value).toContain('othra_pt_secret');
    f.forget();
    expect(f.snippet.value).toBe('');
  });
});
