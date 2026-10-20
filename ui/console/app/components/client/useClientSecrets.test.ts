import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ClientKind, ClientSchema, ClientSecretSchema, SecretGrace } from '../../../gen/client/v1/client_pb';
import { connectError, deferred, setClient, setKnownKeys } from '../../../test/nuxt-globals';
import { graceOptions, secretRows, useClientSecrets } from './useClientSecrets';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const clientId = 'dPm7fmQRJKPVw8PxrgdrN';
const at = (iso: string) => timestampFromDate(new Date(iso));
const current = create(ClientSecretSchema, { id: 'sec_new_000000000000001', hint: 'Xk3Q', createdAt: at('2026-10-20T14:02:00Z') });
const old = create(ClientSecretSchema, { id: 'sec_old_000000000000001', hint: 'p7Lm', createdAt: at('2026-10-01T09:00:00Z'), expiresAt: at('2026-10-21T14:02:00Z') });
const rotated = create(ClientSchema, { id: clientId, kind: ClientKind.WEB, secrets: [old, current] });

describe('secretRows', () => {
  it('puts the current secret first and the old one after it with its end', () => {
    const rows = secretRows([old, current, create(ClientSecretSchema, { id: 'x', hint: 'zzzz' })]);
    expect(rows.map(r => [r.hint, r.old])).toEqual([['Xk3Q', false], ['zzzz', false], ['p7Lm', true]]);
    expect(rows[2]!.expires).toEqual(new Date('2026-10-21T14:02:00Z'));
    expect(rows[1]!.created).toBeNull();
  });

  it('offers Now, 1 hour, 1 day and 7 days', () => {
    expect(graceOptions.map(o => o.value)).toEqual([SecretGrace.NOW, SecretGrace.HOUR, SecretGrace.DAY, SecretGrace.WEEK]);
  });
});

describe('useClientSecrets', () => {
  let generateClientSecret: ReturnType<typeof vi.fn>;
  let rotateClientSecret: ReturnType<typeof vi.fn>;
  let revokeClientSecret: ReturnType<typeof vi.fn>;
  let onClient: ReturnType<typeof vi.fn<(client: unknown) => void>>;

  beforeEach(() => {
    generateClientSecret = vi.fn().mockResolvedValue({ client: create(ClientSchema, { id: clientId, secrets: [current] }), secret: 'othra_cs_first' });
    rotateClientSecret = vi.fn().mockResolvedValue({ client: rotated, secret: 'othra_cs_second' });
    revokeClientSecret = vi.fn().mockResolvedValue({ client: create(ClientSchema, { id: clientId, secrets: [current] }) });
    onClient = vi.fn();
    setClient('$clientClient', { generateClientSecret, rotateClientSecret, revokeClientSecret });
  });

  it('generates the first secret and holds it only until forgotten', async () => {
    const s = useClientSecrets(projectId, clientId, onClient);
    await s.generate();
    expect(generateClientSecret).toHaveBeenCalledWith({ projectId, clientId });
    expect(onClient).toHaveBeenCalledTimes(1);
    expect(s.revealed.value).toEqual({ clientId, secret: 'othra_cs_first' });
    s.forget();
    expect(s.revealed.value).toBeNull();
  });

  it('shows why generating failed', async () => {
    setKnownKeys('apperror.CLT007');
    generateClientSecret.mockRejectedValue(connectError('CLT007'));
    const s = useClientSecrets(projectId, clientId, onClient);
    await s.generate();
    expect(s.error.value?.message).toBe('apperror.CLT007');
    expect(s.revealed.value).toBeNull();
    expect(onClient).not.toHaveBeenCalled();
  });

  it('rotates with the chosen grace, defaulting to a day, and reveals the new secret', async () => {
    const s = useClientSecrets(projectId, clientId, onClient);
    s.askRotate();
    expect([s.rotating.value, s.grace.value]).toEqual([true, SecretGrace.DAY]);
    s.grace.value = SecretGrace.NOW;
    await s.confirmRotate();
    expect(rotateClientSecret).toHaveBeenCalledWith({ projectId, clientId, grace: SecretGrace.NOW });
    expect(s.rotating.value).toBe(false);
    expect(onClient).toHaveBeenCalledWith(rotated);
    expect(s.revealed.value?.secret).toBe('othra_cs_second');
  });

  it('keeps the rotate dialog open with the reason, and closes it on cancel', async () => {
    setKnownKeys('apperror.CLT005');
    rotateClientSecret.mockRejectedValue(connectError('CLT005'));
    const s = useClientSecrets(projectId, clientId, onClient);
    await s.confirmRotate();
    expect(rotateClientSecret).not.toHaveBeenCalled();
    s.askRotate();
    await s.confirmRotate();
    expect(s.rotating.value).toBe(true);
    expect(s.error.value?.message).toBe('apperror.CLT005');
    s.cancelRotate();
    expect([s.rotating.value, s.error.value]).toEqual([false, null]);
  });

  it('revokes the old secret after a confirm', async () => {
    const s = useClientSecrets(projectId, clientId, onClient);
    await s.confirmRevoke();
    expect(revokeClientSecret).not.toHaveBeenCalled();
    const [, oldRow] = secretRows([old, current]);
    s.askRevoke(oldRow!);
    await s.confirmRevoke();
    expect(revokeClientSecret).toHaveBeenCalledWith({ projectId, clientId, secretId: old.id });
    expect(s.revoking.value).toBeNull();
    expect(onClient).toHaveBeenCalledTimes(1);
  });

  it('keeps the revoke confirm open with the reason, and closes it on cancel', async () => {
    setKnownKeys('apperror.CLT006');
    revokeClientSecret.mockRejectedValue(connectError('CLT006'));
    const s = useClientSecrets(projectId, clientId, onClient);
    s.askRevoke(secretRows([old])[0]!);
    await s.confirmRevoke();
    expect(s.error.value?.message).toBe('apperror.CLT006');
    expect(s.revoking.value).not.toBeNull();
    s.cancelRevoke();
    expect(s.revoking.value).toBeNull();
  });

  it('ignores actions and cancels while one runs, and a reply without a client', async () => {
    const pending = deferred<{ client?: unknown, secret: string }>();
    generateClientSecret.mockReturnValue(pending.promise);
    const s = useClientSecrets(projectId, clientId, onClient);
    const first = s.generate();
    await s.generate();
    s.askRotate();
    await s.confirmRotate();
    s.cancelRotate();
    expect(s.rotating.value).toBe(true);
    s.askRevoke(secretRows([old])[0]!);
    await s.confirmRevoke();
    s.cancelRevoke();
    expect(s.revoking.value).not.toBeNull();
    pending.resolve({ secret: 'othra_cs_x' });
    await first;
    expect(generateClientSecret).toHaveBeenCalledTimes(1);
    expect(rotateClientSecret).not.toHaveBeenCalled();
    expect(revokeClientSecret).not.toHaveBeenCalled();
    expect(onClient).not.toHaveBeenCalled();
  });
});
