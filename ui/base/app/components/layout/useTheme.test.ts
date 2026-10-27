import { beforeEach, describe, expect, it, vi } from 'vitest';

import { appConfig, fakeStorage } from '#test/nuxt-globals';
import { themeFamilies, useTheme } from './useTheme';

describe('useTheme', () => {
  let storage: ReturnType<typeof fakeStorage>;
  let setAttribute: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    appConfig.theme = { storageKey: 'othra_theme', followSystem: false };
    storage = fakeStorage();
    setAttribute = vi.fn();
    vi.stubGlobal('localStorage', storage);
    vi.stubGlobal('document', { documentElement: { setAttribute } });
  });

  function applied() {
    return setAttribute.mock.calls.at(-1)?.[1];
  }

  it('defaults to the first family in its default mode', () => {
    const theme = useTheme();
    theme.init();
    expect(theme.family.value.id).toBe('othra');
    expect(theme.mode.value).toBe('light');
    expect(theme.supportsMode.value).toBe(true);
    expect(theme.families).toBe(themeFamilies);
    expect(applied()).toBe('othra-light');
    expect(JSON.parse(storage.data.get('othra_theme')!)).toEqual({ family: 'othra', mode: 'light' });
  });

  it('restores a saved family and mode', () => {
    storage.data.set('othra_theme', JSON.stringify({ family: 'retro', mode: 'dark' }));
    const theme = useTheme();
    theme.init();
    expect(theme.family.value.id).toBe('retro');
    expect(theme.mode.value).toBe('dark');
    expect(applied()).toBe('retro-dark');
  });

  it('uses the family default for a saved mode it lacks or no mode', () => {
    storage.data.set('othra_theme', JSON.stringify({ family: 'night', mode: 'light' }));
    const theme = useTheme();
    theme.init();
    expect(theme.mode.value).toBe('dark');
    storage.data.set('othra_theme', JSON.stringify({ family: 'pastel' }));
    theme.init();
    expect(theme.mode.value).toBe('light');
    expect(applied()).toBe('pastel-light');
  });

  it('ignores an unknown saved family and corrupt storage', () => {
    storage.data.set('othra_theme', JSON.stringify({ family: 'gone', mode: 'dark' }));
    const theme = useTheme();
    theme.init();
    expect(theme.family.value.id).toBe('othra');
    storage.data.set('othra_theme', '{bad');
    theme.init();
    expect(theme.mode.value).toBe('light');
    expect(applied()).toBe('othra-light');
  });

  it('switches family and mode, ignoring unknown values', () => {
    const theme = useTheme();
    theme.setFamily('coffee');
    expect(theme.family.value.id).toBe('coffee');
    expect(theme.mode.value).toBe('dark');
    expect(theme.supportsMode.value).toBe(false);
    theme.setMode('light');
    theme.toggle();
    expect(theme.mode.value).toBe('dark');
    theme.setFamily('nope');
    expect(theme.family.value.id).toBe('coffee');
    theme.setFamily('bumblebee');
    theme.toggle();
    expect(theme.mode.value).toBe('dark');
    expect(applied()).toBe('bumblebee-dark');
    theme.toggle();
    expect(applied()).toBe('bumblebee-light');
  });

  it('shares state between callers', () => {
    useTheme().setFamily('luxury');
    expect(useTheme().family.value.id).toBe('luxury');
  });

  it('keeps an app\'s own key and follows the OS until the person picks', () => {
    let listener: ((e: { matches: boolean }) => void) | undefined;
    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: q === '(prefers-color-scheme: dark)',
      addEventListener: (_: string, fn: (e: { matches: boolean }) => void) => {
        listener = fn;
      },
    }));
    storage.data.set('othra_theme', JSON.stringify({ family: 'retro', mode: 'light' }));
    appConfig.theme = { storageKey: 'othra_portal_theme', followSystem: true };
    const switcher = useTheme();
    const theme = useTheme();
    theme.init();
    expect(switcher.mode.value, 'a component that calls useTheme first changes nothing').toBe('dark');
    expect(theme.family.value.id).toBe('othra');
    expect(applied()).toBe('othra-dark');
    expect(storage.data.has('othra_portal_theme'), 'the first load stores nothing').toBe(false);
    listener!({ matches: false });
    expect(applied(), 'an OS change repaints').toBe('othra-light');
    expect(storage.data.has('othra_portal_theme')).toBe(false);
    useTheme().toggle();
    expect(JSON.parse(storage.data.get('othra_portal_theme')!), 'an explicit pick is stored').toEqual({ family: 'othra', mode: 'dark' });
    listener!({ matches: false });
    expect(applied(), 'and wins over the OS').toBe('othra-dark');
    expect(JSON.parse(storage.data.get('othra_theme')!), 'the console key is untouched').toEqual({ family: 'retro', mode: 'light' });
    useTheme().init();
    expect(theme.mode.value, 'a saved pick beats the OS on the next load').toBe('dark');
  });

  it('survives storage that throws', () => {
    vi.stubGlobal('localStorage', { getItem: () => {
      throw new Error('denied');
    }, setItem: () => {
      throw new Error('denied');
    } });
    const theme = useTheme();
    theme.init();
    expect(applied()).toBe('othra-light');
  });
});
