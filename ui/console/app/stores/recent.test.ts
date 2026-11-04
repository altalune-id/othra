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

  it('keeps working in memory when the browser refuses to store', () => {
    storage.setItem.mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError');
    });
    const recent = useRecentStore();
    expect(() => recent.visit('acme', 'web')).not.toThrow();
    recent.visit('beta');
    expect(recent.orgs).toEqual(['beta', 'acme']);
    expect(recent.lastProject('acme')).toBe('web');
  });

  it('keeps only lists of slugs from a saved value of the wrong shape', () => {
    storage.data.set('othra_recent', JSON.stringify({ orgs: ['acme', 3], projects: { acme: 'web', beta: ['api', null] } }));
    const recent = useRecentStore();
    expect(recent.orgs).toEqual(['acme']);
    expect(recent.lastProject('acme')).toBe('');
    expect(() => recent.visit('acme', 'web')).not.toThrow();
    expect(recent.lastProject('acme')).toBe('web');
  });

  it('drops a corrupt old pin once instead of retrying it on every load', () => {
    storage.data.set('othra_pinned', '{not json');
    expect(useRecentStore().lastOrg()).toBe('');
    expect(storage.data.has('othra_pinned')).toBe(false);
    expect(saved()).toEqual({ orgs: [], projects: {} });
  });

  it('forgets the projects of an org that falls off the kept list', () => {
    const recent = useRecentStore();
    recent.visit('org-0', 'p');
    for (let i = 1; i <= 20; i++) {
      recent.visit(`org-${i}`);
    }
    expect(recent.orgs).toHaveLength(20);
    expect(recent.orgs).not.toContain('org-0');
    expect(recent.projects).not.toHaveProperty('org-0');
  });

  it('orders a list by recency and keeps unopened items in their own order after it', () => {
    const recent = useRecentStore();
    recent.visit('c');
    recent.visit('a');
    expect(recent.byRecency(['a', 'b', 'c', 'd'], s => s, recent.orgs)).toEqual(['a', 'c', 'b', 'd']);
  });
});
