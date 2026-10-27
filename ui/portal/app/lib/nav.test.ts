import { describe, expect, it } from 'vitest';

import { navItems, screenPath } from './nav';

const t = (key: string) => key;

describe('nav', () => {
  it('marks the current screen, nested routes included', () => {
    const active = (path: string) => navItems(path, t).filter(i => i.active).map(i => i.key);
    expect(active('/')).toEqual(['profile']);
    expect(active('/tokens/new')).toEqual(['tokens']);
    expect(active('/sessions')).toEqual(['sessions']);
    expect(navItems('/', t)[1]).toMatchObject({ label: 'nav.sign_in', icon: 'lucide:log-in' });
  });

  it('maps a key back to its path', () => {
    expect(screenPath('apps')).toBe('/apps');
    expect(screenPath('nope')).toBe('/');
  });
});

describe('guard: nav icons', () => {
  it('are bundled, since the client may never fetch icons (CSP connect-src self)', async () => {
    const { readFileSync } = await import('node:fs');
    const config = readFileSync(new URL('../../nuxt.config.ts', import.meta.url), 'utf8');
    for (const s of (await import('./nav')).screens) {
      expect(config).toContain(`'${s.icon}'`);
    }
  });
});
