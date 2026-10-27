import { describe, expect, it } from 'vitest';

import { expectSameKeysAndParams, paramsOf } from '#test/locales';
import baseEnUS from '../../base/i18n/locales/en-US.json';
import enUS from './locales/en-US.json';
import idID from './locales/id-ID.json';

const locales: Record<string, Record<string, string>> = { 'en-US': enUS, 'id-ID': idID };

describe('portal locales', () => {
  it('have the same keys and the same params per key', () => {
    expectSameKeysAndParams(locales);
  });

  it('compile every message', () => {
    for (const messages of Object.values(locales)) {
      Object.values(messages).forEach(paramsOf);
    }
  });

  it('leave the shared keys to the base layer, except the portal\'s own wording', () => {
    const overrides = ['methods.only_method'];
    expect(Object.keys(enUS).filter(key => key in baseEnUS && !overrides.includes(key))).toEqual([]);
    expect(enUS['methods.only_method'], 'M8 design 13').toBe('This is your only way to sign in.');
  });
});

describe('session copy (M8 R-5)', () => {
  it('never promises that ending a session cuts every app off at once', () => {
    for (const messages of Object.values(locales)) {
      for (const key of ['sessions.sign_out_body', 'sessions.others_body']) {
        expect(messages[key], key).toMatch(/15/);
      }
    }
    expect(enUS['sessions.sign_out_body']).toContain('except apps you allowed to stay connected');
  });
});
