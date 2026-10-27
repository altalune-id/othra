import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const template = readFileSync(new URL('./PersonalKeyCreateForm.vue', import.meta.url), 'utf8').split('<template>').slice(1).join('<template>');

describe('the personal key create form', () => {
  it('labels every text input and select through a matching label', () => {
    const controls = [...template.matchAll(/<(?:select|input)\b[^>]*>/g)]
      .map(m => m[0])
      .filter(tag => tag.startsWith('<select') || /type="text"/.test(tag));
    expect(controls.length).toBeGreaterThanOrEqual(3);
    const labelled = new Set([...template.matchAll(/<label\b[^>]*:for="([^"]+)"/g)].map(m => m[1]));
    for (const tag of controls) {
      const id = /:id="([^"]+)"/.exec(tag)?.[1];
      expect(id, `${tag} needs an id`).toBeTruthy();
      expect(labelled.has(id!), `${tag} needs <label :for="${id}">`).toBe(true);
    }
  });

  it('labels each checkbox and radio by wrapping it in its label', () => {
    const wrapped = [...template.matchAll(/<label\s[^>]*>\s*<input\s[^>]*type="(?:checkbox|radio)"/g)].length;
    const all = [...template.matchAll(/<input\s[^>]*type="(?:checkbox|radio)"/g)].length;
    expect(all).toBeGreaterThan(0);
    expect(wrapped).toBe(all);
  });
});
