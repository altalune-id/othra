import { describe, expect, it } from 'vitest';

import { expectSameKeysAndParams, paramsOf } from '#test/locales';
import baseEnUS from '../../base/i18n/locales/en-US.json';
import baseIdID from '../../base/i18n/locales/id-ID.json';
import enUS from './locales/en-US.json';
import idID from './locales/id-ID.json';

const locales: Record<string, Record<string, string>> = { 'en-US': enUS, 'id-ID': idID };

describe('console locales', () => {
  it('have the same keys and the same params per key', () => {
    expectSameKeysAndParams(locales);
  });

  it('compile every message', () => {
    for (const messages of Object.values(locales)) {
      Object.values(messages).forEach(paramsOf);
    }
  });

  it('leave the shared keys to the base layer', () => {
    expect(Object.keys(enUS).filter(key => key in baseEnUS)).toEqual([]);
  });
});

describe('console copy', () => {
  it('writes "upstream IdP" mid-sentence as the glossary does', () => {
    expect(enUS['endusers.methods_help']).toContain('each linked upstream IdP');
  });

  it('says "berkas", not "file", for a stored file in id-ID', () => {
    const id: Record<string, string> = { ...idID, ...baseIdID };
    expect(Object.entries(id).filter(([, text]) => /\bfile\b/i.test(text)).map(([key]) => key)).toEqual([]);
  });
});
