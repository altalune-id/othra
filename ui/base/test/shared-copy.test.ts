import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { tokenWhen } from '../app/lib/account/views';
import enUS from '../i18n/locales/en-US.json';

const messages = enUS as Record<string, string>;
const dir = fileURLToPath(new URL('../app/components/account/', import.meta.url));

describe('guard: shared account copy lives in the base layer', () => {
  it('has every key tokenWhen returns, so both apps can use it', () => {
    const keys = [
      tokenWhen({ status: 'revoked', expiry: { kind: 'none' } }).key,
      tokenWhen({ status: 'expired', expiry: { kind: 'expired' } }).key,
      tokenWhen({ status: 'active', expiry: { kind: 'soon', days: 3 } }).key,
      tokenWhen({ status: 'active', expiry: { kind: 'ok' } }).key,
    ];
    expect(keys.filter(key => !(key in messages))).toEqual([]);
  });

  it('has every key the shared rows use', () => {
    const used = readdirSync(dir).flatMap(name => [...readFileSync(join(dir, name), 'utf8').matchAll(/\bt\('([\w.]+)'/g)].map(m => m[1]!));
    used.push('tokens.status_active', 'tokens.status_expired', 'tokens.status_revoked', 'sessions.sign_out_title', 'apps.disconnect_title', 'tokens.revoke_title', 'tokens.revoke_body', 'common.loading');
    expect(used.length).toBeGreaterThan(10);
    expect(used.filter(key => !(key in messages))).toEqual([]);
  });
});
