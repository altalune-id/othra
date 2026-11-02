import { beforeEach, describe, expect, it, vi } from 'vitest';
import { enhance, langCookie } from './switcher';

const select = () => document.querySelector<HTMLSelectElement>('[data-lang-select]')!;

describe('langCookie', () => {
  it('shares the console cookie on Path=/ for a year and adds Secure on https', () => {
    expect(langCookie('id-ID', true)).toBe('othra_lang=id-ID; Path=/; Max-Age=31536000; SameSite=Lax; Secure');
    expect(langCookie('en-US', false)).toBe('othra_lang=en-US; Path=/; Max-Age=31536000; SameSite=Lax');
  });
});

describe('enhance', () => {
  beforeEach(() => {
    document.cookie = 'othra_lang=; Max-Age=0; Path=/';
    document.body.innerHTML = `
      <select data-lang-select hidden>
        <option value="en-US" selected>English</option>
        <option value="id-ID">Bahasa Indonesia</option>
      </select>`;
  });

  it('shows the select, then sets the cookie and reloads on change', () => {
    const reload = vi.fn();
    enhance(document, { protocol: 'http:', reload });
    expect(select().hidden).toBe(false);

    select().value = 'id-ID';
    select().dispatchEvent(new Event('change'));
    expect(document.cookie).toContain('othra_lang=id-ID');
    expect(reload).toHaveBeenCalledOnce();
  });

  it('does nothing on a page without the select', () => {
    document.body.innerHTML = '';
    expect(() => enhance(document, { protocol: 'http:', reload: vi.fn() })).not.toThrow();
  });
});
