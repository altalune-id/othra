import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const app = fileURLToPath(new URL('../..', import.meta.url));

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return sources(path);
    }
    return /\.(?:vue|ts)$/.test(entry.name) && !entry.name.endsWith('.test.ts') ? [path] : [];
  });
}

describe('console markup', () => {
  const files = sources(app).map(path => ({ path: path.slice(app.length), text: readFileSync(path, 'utf8') }));

  it('finds the console sources', () => {
    expect(files.length).toBeGreaterThan(50);
  });

  it('uses no daisyUI 4 bordered or small form controls', () => {
    const offenders = files
      .filter(f => /\b(?:input|select|textarea|file-input)-(?:bordered|sm|xs)\b/.test(f.text))
      .map(f => f.path);
    expect(offenders).toEqual([]);
  });
});

describe('light-theme contrast', () => {
  it('keeps text neutral: error, warning, success and info colours go on icons only', () => {
    const files = sources(app).map(path => ({ path: path.slice(app.length), text: readFileSync(path, 'utf8') }));
    const offenders = files
      .filter(f => /<(?:p|span|div|dd|li|label)\s[^>]*\bclass="[^"]*\btext-(?:error|warning|success|info)\b/.test(f.text)
        || /<(?:p|span|div|dd|li|label)\s[^>]*:class="[^"]*'text-(?:error|warning|success|info)'/.test(f.text))
      .map(f => f.path);
    expect(offenders).toEqual([]);
  });
});

describe('org create modal copy', () => {
  it('takes its placeholders and the random-slug help from the locales', () => {
    const modal = readFileSync(join(app, 'components/org/create/OrgCreateModal.vue'), 'utf8');
    expect(modal).not.toContain('generated when empty');
    expect(modal).toContain('t(\'org.slug_placeholder\')');
    expect(modal).toContain('t(\'org.slug_random_help\')');
  });
});

describe('m1.5 design re-check (console-wide)', () => {
  const files = sources(app).map(path => ({ path: path.slice(app.length), text: readFileSync(path, 'utf8') }));

  it('keeps destructive ghost buttons neutral with a red icon (O4)', () => {
    const offenders = files
      .filter(f => /<(?:button|NuxtLink|label|a)\s[^>]*\bclass="(?:[^"]*\s)?btn\s(?:[^"]*\s)?text-(?:error|warning|success|info)\b/.test(f.text))
      .map(f => f.path);
    expect(offenders).toEqual([]);
  });

  it('gives the small checkboxes square corners so they do not look like radios (P-a)', () => {
    for (const path of ['components/webhook/create/WebhookCreateModal.vue', 'components/apikey/create/APIKeyCreateModal.vue']) {
      const text = files.find(f => f.path === path)!.text;
      for (const box of text.match(/class="checkbox [^"]*"/g) ?? []) {
        expect(box, path).toContain('rounded-sm');
      }
    }
  });

  it('collapses member row actions into a menu below sm (O8)', () => {
    const list = files.find(f => f.path === 'components/member/MemberList.vue')!.text;
    expect(list).toMatch(/class="dropdown dropdown-left sm:hidden"/);
    expect(list).toContain('lucide:ellipsis');
    expect(list).toMatch(/class="[^"]*max-sm:hidden/);
  });

  it('opens AppModal as a native modal dialog so Esc closes it (product M1.5 7)', () => {
    const modal = files.find(f => f.path === 'components/ui/AppModal.vue')!.text;
    expect(modal).toContain('showModal()');
    expect(modal).toContain('@cancel.prevent="cancel()"');
    expect(modal).toMatch(/function cancel\(\) \{\s+if \(dismissible\) \{\s+emit\('close'\);/);
    expect(modal).toContain('<form v-if="dismissible" method="dialog" class="modal-backdrop"');
    expect(modal).not.toContain('modal-open');
  });

  it('darkens the othra-dark primary for button text contrast (O3)', () => {
    const theme = readFileSync(join(app, '../../theme.css'), 'utf8');
    const dark = theme.slice(theme.indexOf('name: \'othra-dark\''));
    expect(dark.slice(0, dark.indexOf('}'))).toContain('--color-primary: oklch(55% 0.233 277.117);');
  });
});

describe('final M1.5 nits', () => {
  it('names every AppModal by its title', () => {
    const modal = readFileSync(join(app, 'components/ui/AppModal.vue'), 'utf8');
    expect(modal).toContain(':aria-labelledby="title ? titleId : undefined"');
    expect(modal).toContain('<h3 v-if="title" :id="titleId"');
  });

  it('says "berkas", not "file", for a stored file in id-ID', () => {
    const id = JSON.parse(readFileSync(join(app, '../i18n/locales/id-ID.json'), 'utf8')) as Record<string, string>;
    const offenders = Object.entries(id).filter(([, text]) => /\bfile\b/i.test(text)).map(([key]) => key);
    expect(offenders).toEqual([]);
  });
});

describe('member-visible write actions', () => {
  const read = (path: string) => readFileSync(join(app, path), 'utf8');

  it('shows org API key Revoke only to people who manage keys, behind a confirm naming the key', () => {
    const list = read('components/apikey/list/APIKeyList.vue');
    expect(list).toMatch(/<button v-if="canManage && !k\.revokedAt"[^>]*@click="askRevoke\(k\)"/);
    expect(list).toContain('<AppModal :open="!!revoking" :title="t(\'apikeys.revoke_confirm\')"');
    expect(list).toContain('apikeys.revoke_body');
    expect(read('pages/o/[org]/settings/api-keys.vue')).toContain('<APIKeyList :org-id="org.id" :can-manage="canManage" />');
  });

  it('asks before deleting a webhook', () => {
    const list = read('components/webhook/list/WebhookList.vue');
    expect(list).toMatch(/@click="askDelete\(webhook\)"/);
    expect(list).toContain('<AppModal :open="!!deleting" :title="t(\'webhooks.delete_title\')"');
  });

  it('makes org settings read-only for people without org:write', () => {
    const settings = read('pages/o/[org]/settings/index.vue');
    expect(settings).toContain('useCan(\'org:write\')');
    expect(settings).toMatch(/<form v-if="canEdit" novalidate/);
    expect(settings).toContain('<dl v-else');
    expect(settings).toContain('settings.read_only');
  });
});
