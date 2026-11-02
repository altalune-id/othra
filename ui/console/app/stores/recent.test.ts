import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { fakeStorage } from '#test/nuxt-globals';
import { useRecentStore } from './recent';

describe('useRecentStore', () => {
  let storage: ReturnType<typeof fakeStorage>;

  beforeEach(() => {
    setActivePinia(createPinia());
    storage = fakeStorage();
    vi.stubGlobal('localStorage', storage);
  });

  function saved() {
    return JSON.parse(storage.data.get('othra_recent') ?? 'null');
  }

  it('starts empty with nothing saved', () => {
    const recent = useRecentStore();
    expect(recent.lastOrg()).toBe('');
    expect(recent.lastProject('acme')).toBe('');
  });

  it('turns the old pins into the first last-opened entries once, then deletes them', () => {
    storage.data.set('othra_pinned', JSON.stringify({ org: 'acme', projects: { acme: 'web', beta: 'api' } }));
    const recent = useRecentStore();
    expect(recent.lastOrg()).toBe('acme');
    expect(recent.lastProject('acme')).toBe('web');
    expect(recent.lastProject('beta')).toBe('api');
    expect(storage.data.has('othra_pinned')).toBe(false);
    expect(saved()).toEqual({ orgs: ['acme'], projects: { acme: ['web'], beta: ['api'] } });
  });

  it('reads the saved list and leaves old pins alone once it exists', () => {
    storage.data.set('othra_recent', JSON.stringify({ orgs: ['beta', 'acme'], projects: { beta: ['api'] } }));
    storage.data.set('othra_pinned', JSON.stringify({ org: 'acme' }));
    const recent = useRecentStore();
    expect(recent.lastOrg()).toBe('beta');
    expect(recent.lastProject('beta')).toBe('api');
  });

  it('ignores corrupt saved data', () => {
    storage.data.set('othra_recent', '{not json');
    expect(useRecentStore().lastOrg()).toBe('');
  });

  it('moves each visit to the front, once, and saves it', () => {
    const recent = useRecentStore();
    recent.visit('acme', 'web');
    recent.visit('beta');
    recent.visit('acme', 'api');
    recent.visit('acme', 'web');
    expect(recent.orgs).toEqual(['acme', 'beta']);
    expect(saved().projects.acme).toEqual(['web', 'api']);
  });

  it('orders a list by recency and keeps unopened items in their own order after it', () => {
    const recent = useRecentStore();
    recent.visit('c');
    recent.visit('a');
    expect(recent.byRecency(['a', 'b', 'c', 'd'], s => s, recent.orgs)).toEqual(['a', 'c', 'b', 'd']);
  });
});
