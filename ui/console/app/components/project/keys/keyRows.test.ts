import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { SigningKeySchema, SigningKeyStatus } from '#gen/keys/v1/keys_pb';
import { t } from '#test/nuxt-globals';
import { keyRows } from './keyRows';

const at = (d: Date) => timestampFromDate(d);
const now = new Date(2026, 9, 18, 14, 22);
const day = (d: Date) => new Intl.DateTimeFormat('en-US', { dateStyle: 'medium' }).format(d);
const time = (d: Date) => new Intl.DateTimeFormat('en-US', { timeStyle: 'short' }).format(d);

describe('keyRows', () => {
  it('formats each key for the table: short kid, status, help line, created and retires', () => {
    const created = new Date(2026, 9, 10);
    const activates = new Date(2026, 9, 18, 14, 32);
    const retireAfter = new Date(2026, 9, 24);
    const keys = [
      create(SigningKeySchema, { publicId: 'pk1', kid: '3f9a1c2e9d8b', alg: 'RS256', status: SigningKeyStatus.NEXT, createdAt: at(now), activatesAt: at(activates) }),
      create(SigningKeySchema, { publicId: 'pk2', kid: '8b20d4f1aa00', alg: 'RS256', status: SigningKeyStatus.ACTIVE, createdAt: at(created) }),
      create(SigningKeySchema, { publicId: 'pk3', kid: '1c77e0aa1234', alg: 'RS256', status: SigningKeyStatus.RETIRING, createdAt: at(created), retireAfter: at(retireAfter) }),
      create(SigningKeySchema, { publicId: 'pk4', kid: '9e01b3c55555', alg: 'RS256', status: SigningKeyStatus.RETIRED, createdAt: at(created), retireAfter: at(retireAfter) }),
      create(SigningKeySchema, { publicId: 'pk5', kid: 'ffff', alg: 'RS256', status: SigningKeyStatus.UNSPECIFIED }),
    ];
    expect(keyRows(keys, { t, locale: 'en-US', now })).toEqual([
      { id: 'pk1', kid: '3f9a1c2e9d8b', short: '3f9a1c2e', alg: 'RS256', status: 'next', help: `keys.next_help|{"time":"${time(activates)}"}`, created: day(now), retires: '' },
      { id: 'pk2', kid: '8b20d4f1aa00', short: '8b20d4f1', alg: 'RS256', status: 'active', help: '', created: day(created), retires: '' },
      { id: 'pk3', kid: '1c77e0aa1234', short: '1c77e0aa', alg: 'RS256', status: 'retiring', help: `keys.retiring_help|{"date":"${day(retireAfter)}"}`, created: day(created), retires: day(retireAfter) },
      { id: 'pk4', kid: '9e01b3c55555', short: '9e01b3c5', alg: 'RS256', status: 'retired', help: '', created: day(created), retires: day(retireAfter) },
      { id: 'pk5', kid: 'ffff', short: 'ffff', alg: 'RS256', status: 'unknown', help: '', created: '', retires: '' },
    ]);
  });
});
