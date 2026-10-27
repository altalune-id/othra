import type { SignInMethod } from '#gen/account/v1/account_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { SignInMethodKind, SignInMethodLock } from '#gen/account/v1/account_pb';

export type MethodKey = 'email_link' | 'google';

export interface MethodRow {
  key: MethodKey;
  labelKey: string;
  email: string;
  linked: boolean;
  linkedAt?: Date;
  identityId: string;
  canRemove: boolean;
  lockKey: string;
}

const order: Record<MethodKey, number> = { email_link: 0, google: 1 };

// methodRows turns the server's sign-in methods into display rows, email link first; it never re-derives whether a method can go.
export function methodRows(methods: SignInMethod[]): MethodRow[] {
  const rows: MethodRow[] = [];
  for (const m of methods) {
    const key: MethodKey | undefined = m.kind === SignInMethodKind.EMAIL_LINK ? 'email_link' : m.kind === SignInMethodKind.GOOGLE ? 'google' : undefined;
    if (!key) {
      continue;
    }
    rows.push({
      key,
      labelKey: key === 'google' ? 'methods.google' : 'methods.email',
      email: m.email,
      linked: m.linked,
      linkedAt: m.linkedAt ? timestampDate(m.linkedAt) : undefined,
      identityId: m.identityId,
      canRemove: m.canRemove,
      lockKey: m.lock === SignInMethodLock.LAST_METHOD ? 'methods.only_method' : '',
    });
  }
  return rows.sort((a, b) => order[a.key] - order[b.key]);
}

// methodEmail is the address a linked method row shows; the email link always uses the account's own address.
export function methodEmail(row: Pick<MethodRow, 'key' | 'email'>, accountEmail: string): string {
  return row.email || (row.key === 'email_link' ? accountEmail : '');
}
