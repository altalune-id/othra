import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const template = readFileSync(new URL('./PersonalKeyCreateForm.vue', import.meta.url), 'utf8').split('<template>').slice(1).join('<template>');

describe('the personal key create form', () => {
  it('labels every text input and select, by wrapping label or a matching :for', () => {
    const controls = [...template.matchAll(/<(?:select|input)\s[^>]*>/g)]
      .filter(m => m[0].startsWith('<select') || /type="text"/.test(m[0]));
    expect(controls.length).toBeGreaterThanOrEqual(3);
    const wrappers = [...template.matchAll(/<label\s[^>]*>[\s\S]*?<\/label>/g)].map(m => [m.index, m.index + m[0].length]);
    const labelled = new Set([...template.matchAll(/<label\s[^>]*:for="([^"]+)"/g)].map(m => m[1]));
    for (const m of controls) {
      const id = /:id="([^"]+)"/.exec(m[0])?.[1];
      const wrapped = wrappers.some(([from, to]) => m.index > from! && m.index < to!);
      expect(wrapped || labelled.has(id!), `${m[0]} needs a label`).toBe(true);
    }
  });

  it('labels each checkbox and radio by wrapping it in its label', () => {
    const wrapped = [...template.matchAll(/<label\s[^>]*>\s*<input\s[^>]*type="(?:checkbox|radio)"/g)].length;
    const all = [...template.matchAll(/<input\s[^>]*type="(?:checkbox|radio)"/g)].length;
    expect(all).toBeGreaterThan(0);
    expect(wrapped).toBe(all);
  });
});
