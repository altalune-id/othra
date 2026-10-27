import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { SignInMethodKind, SignInMethodLock, SignInMethodSchema } from '#gen/account/v1/account_pb';
import { methodEmail, methodRows } from './methods';

describe('methodRows', () => {
  it('lists the email link first and passes the server guard through untouched', () => {
    const at = new Date('2026-10-20T00:00:00Z');
    const rows = methodRows([
      create(SignInMethodSchema, { kind: SignInMethodKind.GOOGLE, linked: true, identityId: 'J2xKv7NqP4wRt9LmZ6cBh', email: 'ana.s@gmail.com', linkedAt: timestampFromDate(at), canRemove: false, lock: SignInMethodLock.LAST_METHOD }),
      create(SignInMethodSchema, { kind: SignInMethodKind.EMAIL_LINK, linked: true, email: 'ana@acme.test' }),
      create(SignInMethodSchema, { kind: SignInMethodKind.UNSPECIFIED }),
    ]);
    expect(rows.map(r => r.key)).toEqual(['email_link', 'google']);
    expect(rows[0]).toMatchObject({ labelKey: 'methods.email', email: 'ana@acme.test', lockKey: '', linkedAt: undefined });
    expect(rows[1]).toMatchObject({ labelKey: 'methods.google', identityId: 'J2xKv7NqP4wRt9LmZ6cBh', canRemove: false, lockKey: 'methods.only_method', linkedAt: at });
  });

  it('shows an unlinked Google as connectable', () => {
    const [row] = methodRows([create(SignInMethodSchema, { kind: SignInMethodKind.GOOGLE, linked: false, canRemove: false })]);
    expect(row).toMatchObject({ key: 'google', linked: false, lockKey: '' });
  });
});

describe('methodEmail', () => {
  it('shows a linked upstream IdP\'s own address', () => {
    expect(methodEmail({ key: 'google', email: 'ada@gmail.com' }, 'ada@example.com')).toBe('ada@gmail.com');
  });

  it('falls back to the account address for the email link only', () => {
    expect(methodEmail({ key: 'email_link', email: '' }, 'ada@example.com')).toBe('ada@example.com');
    expect(methodEmail({ key: 'google', email: '' }, 'ada@example.com')).toBe('');
  });
});
