import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { ClientSecretSchema, SecretGrace } from '#gen/client/v1/client_pb';
import { graceOptions, secretRows } from './useSecrets';

const at = (iso: string) => timestampFromDate(new Date(iso));
const current = create(ClientSecretSchema, { id: 'sec_new_000000000000001', hint: 'Xk3Q', createdAt: at('2026-10-20T14:02:00Z') });
const old = create(ClientSecretSchema, { id: 'sec_old_000000000000001', hint: 'p7Lm', createdAt: at('2026-10-01T09:00:00Z'), expiresAt: at('2026-10-21T14:02:00Z') });

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
