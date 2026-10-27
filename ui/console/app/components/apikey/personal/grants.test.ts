import type { Scope } from './grants';

import { describe, expect, it } from 'vitest';
import { grants, mintableForMe } from './grants';

describe('grants', () => {
  it('matches like scope.Grants in Go', () => {
    expect(grants(['*'], 'webhook:read')).toBe(true);
    expect(grants(['webhook:*'], 'webhook:read')).toBe(true);
    expect(grants(['webhook:*'], 'member:read')).toBe(false);
    expect(grants(['webhook:read'], 'webhook:read')).toBe(true);
    expect(grants(['webhook:read'], 'webhook:write')).toBe(false);
    expect(grants([], 'webhook:read')).toBe(false);
    expect(grants(['*'], '')).toBe(false);
  });
});

describe('mintableForMe', () => {
  const catalog: Scope[] = [
    { name: 'org:write', level: 'org', labelKey: 'scope.org_write' },
    { name: 'webhook:read', level: 'project', labelKey: 'scope.webhook_read' },
    { name: 'webhook:write', level: 'project', labelKey: 'scope.webhook_write' },
    { name: 'apikey:write', level: 'org', labelKey: 'scope.apikey_write', personOnly: true },
  ];

  it('drops person-only and above-role scopes', () => {
    expect(mintableForMe(catalog, ['webhook:*', 'apikey:write']).map(s => s.name)).toEqual(['webhook:read', 'webhook:write']);
  });

  it('an owner mints every mintable scope', () => {
    expect(mintableForMe(catalog, ['*']).map(s => s.name)).toEqual(['org:write', 'webhook:read', 'webhook:write']);
  });

  it('nothing without permissions', () => {
    expect(mintableForMe(catalog, [])).toEqual([]);
  });
});
