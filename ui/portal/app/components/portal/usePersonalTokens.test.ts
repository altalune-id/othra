import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { tokenWhen } from '#base/lib/account/views';
import { PersonalTokenSchema, PersonalTokenState, TokenServerSchema } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { usePersonalTokens } from './usePersonalTokens';

const server = create(TokenServerSchema, { id: 'dPm7fmQRJKPVw8PxrgdrN', name: 'Notes MCP', identifier: 'https://mcp.acme.test/mcp', scopes: ['notes:read'] });
const token = create(PersonalTokenSchema, { id: 'K9tWq4ZbN2xLm7VcR5pHs', name: 'cli', hint: 'ab12', server, scopes: ['notes:read'], state: PersonalTokenState.ACTIVE, expiresAt: timestampFromDate(new Date('2026-10-29T00:00:00Z')) });

describe('usePersonalTokens', () => {
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

  it('lists and revokes a token', async () => {
    const l = usePersonalTokens();
    await l.load();
    l.revoke.ask(l.tokens.value[0]!);
    await l.revoke.confirm();
    expect(client.revokePersonalToken).toHaveBeenCalledWith({ tokenId: 'K9tWq4ZbN2xLm7VcR5pHs' });
    expect(l.tokens.value[0]!.status).toBe('revoked');
    expect(l.tokens.value[0]!.revokedAt, 'the row reads "Revoked <date>", never a bare "Revoked"').toBeInstanceOf(Date);
    expect(tokenWhen(l.tokens.value[0]!).date).toBe(l.tokens.value[0]!.revokedAt);
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.listPersonalTokens!.mockRejectedValueOnce(connectError('GEN900'));
    const l = usePersonalTokens();
    await l.load();
    expect(l.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('marks only the revoked token as revoked', async () => {
    const other = create(PersonalTokenSchema, { ...token, id: 'W1zRq6NvK3pLx8TmB4cHd', name: 'ci' });
    client.listPersonalTokens!.mockResolvedValueOnce({ tokens: [token, other] });
    const l = usePersonalTokens();
    await l.load();
    l.revoke.ask(l.tokens.value[1]!);
    await l.revoke.confirm();
    expect(l.tokens.value.map(t => t.status)).toEqual(['active', 'revoked']);
  });
});
