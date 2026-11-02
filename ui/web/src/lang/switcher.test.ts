import { beforeEach, describe, expect, it, vi } from 'vitest';
import { enhance, langCookie } from './switcher';

const select = () => document.querySelector<HTMLSelectElement>('[data-lang-select]')!;

describe('langCookie', () => {
  it('writes the named cookie on Path=/ for a year and adds Secure on https', () => {
    expect(langCookie('pick', 'id-ID', true)).toBe('pick=id-ID; Path=/; Max-Age=31536000; SameSite=Lax; Secure');
    expect(langCookie('pick', 'en-US', false)).toBe('pick=en-US; Path=/; Max-Age=31536000; SameSite=Lax');
  });
});

describe('enhance', () => {
  beforeEach(() => {
    document.cookie = 'pick=; Max-Age=0; Path=/';
    document.body.innerHTML = `
      <select data-lang-select data-cookie="pick" hidden>
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
    expect(document.cookie).toContain('pick=id-ID');
    expect(reload).toHaveBeenCalledOnce();
  });

  it('stays hidden without a cookie name', () => {
    select().removeAttribute('data-cookie');
    enhance(document, { protocol: 'http:', reload: vi.fn() });
    expect(select().hidden).toBe(true);
  });

  it('does nothing on a page without the select', () => {
    document.body.innerHTML = '';
    expect(() => enhance(document, { protocol: 'http:', reload: vi.fn() })).not.toThrow();
  });
});
