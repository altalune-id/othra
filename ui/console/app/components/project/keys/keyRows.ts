import type { SigningKey } from '#gen/keys/v1/keys_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { formatDate } from '#base/lib/format/date';
import { SigningKeyStatus } from '#gen/keys/v1/keys_pb';
import { formatWhen, shortKid } from './useSigningKeys';

export type KeyStatus = 'next' | 'active' | 'retiring' | 'retired' | 'unknown';

export interface KeyRow {
  id: string;
  kid: string;
  short: string;
  alg: string;
  status: KeyStatus;
  help: string;
  created: string;
  retires: string;
}

export interface KeyRowFormat {
  t: (key: string, params?: Record<string, unknown>) => string;
  locale: string;
  now?: Date;
}

const statusNames: Partial<Record<SigningKeyStatus, KeyStatus>> = {
  [SigningKeyStatus.NEXT]: 'next',
  [SigningKeyStatus.ACTIVE]: 'active',
  [SigningKeyStatus.RETIRING]: 'retiring',
  [SigningKeyStatus.RETIRED]: 'retired',
};

// keyRows formats signing keys for the table and the mobile list.
export function keyRows(keys: SigningKey[], { t, locale, now = new Date() }: KeyRowFormat): KeyRow[] {
  return keys.map((k) => {
    const status = statusNames[k.status] ?? 'unknown';
    const retireAfter = k.retireAfter ? timestampDate(k.retireAfter) : undefined;
    let help = '';
    if (status === 'next' && k.activatesAt) {
      help = t('keys.next_help', { time: formatWhen(timestampDate(k.activatesAt), locale, now) });
    } else if (status === 'retiring' && retireAfter) {
      help = t('keys.retiring_help', { date: formatDate(retireAfter, locale) });
    }
    return {
      id: k.publicId,
      kid: k.kid,
      short: shortKid(k.kid),
      alg: k.alg,
      status,
      help,
      created: formatDate(k.createdAt, locale),
      retires: retireAfter && (status === 'retiring' || status === 'retired') ? formatDate(retireAfter, locale) : '',
    };
  });
}
