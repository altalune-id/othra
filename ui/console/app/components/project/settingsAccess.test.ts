import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const pages = ['index', 'issuer', 'branding', 'legal'].map(name => ({
  name,
  source: readFileSync(new URL(`../../pages/o/[org]/projects/[project]/settings/${name}.vue`, import.meta.url), 'utf8'),
}));

describe('project settings pages', () => {
  it.each(pages)('$name only renders an editable form for people who can edit', ({ source }) => {
    const forms = [...source.matchAll(/<form\s[^>]*>/g)].map(m => m[0]);
    expect(forms.length).toBeGreaterThan(0);
    for (const form of forms) {
      expect(form).toContain('v-if="canEdit"');
    }
    expect(source).toContain('useCanEditProject');
  });

  it('hides the publish button from people who cannot edit', () => {
    const legal = pages.find(p => p.name === 'legal')!.source;
    expect(legal).toMatch(/<div v-if="canEdit"[^>]*>\s*<button[^>]*@click="legal\.askBump\(\)"/);
  });

  it('shows the locked issuer note as a note, not an alert', () => {
    const issuer = pages.find(p => p.name === 'issuer')!.source;
    expect(issuer).not.toContain('role="alert"');
  });
});

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8');

describe('project settings markup (M1 review)', () => {
  it.each(pages)('$name tells people who cannot edit why', ({ source }) => {
    expect(source).toContain('<ProjectReadOnlyNote v-if="!canEdit"');
  });

  it.each(pages)('$name keeps light-theme text neutral: no success or warning coloured text', ({ source }) => {
    expect(source).not.toMatch(/<(?:span|p)[^>]*class="[^"]*\btext-(?:success|warning)\b/);
    expect(source).not.toMatch(/alert-(?:info|warning) alert-soft(?![^>]*>\s*<Icon[^>]*class="[^"]*text-)/);
    expect(source).not.toContain('btn-warning btn-outline');
  });

  it('uses the shared SavedNote wherever a form says it saved', () => {
    for (const { name, source } of pages) {
      expect(source, name).not.toContain('{{ t(\'common.saved\') }}');
      expect(source, name).toContain('<SavedNote v-if="saved"');
    }
    expect(read('../ui/SavedNote.vue')).toContain('text-base-content/70');
  });

  it('labels the bump versions and the clear-terms action', () => {
    const legal = pages.find(p => p.name === 'legal')!.source;
    expect(legal).toContain('legal.bump_versions');
    expect(legal).toContain('legal.clear_terms_confirm');
  });

  it('shows the default colour labelled "Default" when no colour is set, not black', () => {
    const branding = pages.find(p => p.name === 'branding')!.source;
    expect(branding).not.toContain('#000000');
    expect(branding).not.toContain('opacity-0');
    expect(branding).toContain('branding.default_colour');
  });

  it('prefixes the handle with /p/ only', () => {
    expect(pages.find(p => p.name === 'issuer')!.source).toContain('prefix="/p/"');
    expect(read('./create/ProjectCreateModal.vue')).toContain('prefix="/p/"');
  });

  it('uses product copy in the create modal, not agent-era placeholders', () => {
    const modal = read('./create/ProjectCreateModal.vue');
    expect(modal).not.toMatch(/support-?bot/i);
    expect(modal).not.toContain('input-bordered');
  });

  it('builds the edit project modal from daisyUI 5 fieldsets', () => {
    const modal = read('./edit/ProjectEditModal.vue');
    expect(modal).not.toMatch(/input-(?:sm|bordered)/);
    expect(modal.match(/class="fieldset"/g)).toHaveLength(2);
    expect(modal.match(/<label :for="ids\.\w+" class="fieldset-legend">/g)).toHaveLength(2);
  });

  it('puts the branding form in a card on a width-capped page, like its sibling pages', () => {
    const branding = pages.find(p => p.name === 'branding')!.source;
    expect(branding).toMatch(/<div class="max-w-5xl space-y-6">/);
    expect(branding).toMatch(/<form v-if="canEdit" novalidate class="card self-start border border-base-300 bg-base-100"/);
  });

  it('calls the slug "Console URL name" on the projects table and edit modal', () => {
    expect(read('../../pages/o/[org]/projects/index.vue')).not.toContain('common.slug');
    expect(read('./edit/ProjectEditModal.vue')).not.toContain('common.slug');
  });

  it('puts the project logo in the Overview header, not the Issuer card title', () => {
    const overview = read('../../pages/o/[org]/projects/[project]/index.vue');
    expect(overview).toMatch(/<img v-if="full\?\.logoUrl"[^>]*class="size-10 [^"]*max-sm:size-8/);
    expect(overview).not.toContain('size-6');
  });
});

describe('m1.5 design re-check', () => {
  it('hides New project and Edit from people who cannot edit projects (O1)', () => {
    const list = read('../../pages/o/[org]/projects/index.vue');
    expect(list).toContain('useCanEditProject');
    expect(list).toMatch(/<AppButton v-if="org && canEdit"[^>]*@click="ui\.openProjectCreate\(\)"/);
    expect(list).toMatch(/<button v-if="canEdit"[^>]*@click="edit\(project\)"/);
  });

  it('puts the logo file input before its label as a peer, so keyboard focus shows on the label (O2)', () => {
    const branding = pages.find(p => p.name === 'branding')!.source;
    expect(branding).toMatch(/<input :id="ids\.logo"[^>]*type="file"[^>]*class="peer sr-only"[^>]*>\s*<label :for="ids\.logo" class="btn btn-sm peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-base-content/);
    expect(branding).not.toContain('has-[:focus-visible]');
  });

  it('puts General in a card on a max-w-2xl page with a plain primary Save (O6)', () => {
    const general = pages.find(p => p.name === 'index')!.source;
    expect(general).toContain('<div v-if="project" class="max-w-2xl space-y-6">');
    expect(general).toMatch(/<section class="card border border-base-300 bg-base-100">\s*<div class="card-body gap-2">/);
    expect(general).toMatch(/<form v-if="canEdit" novalidate/);
    expect(general).toMatch(/<dl v-else /);
    expect(general).not.toContain('text-xs text-base-content/60">{{ t(\'common.id\') }}');
    expect(general).toContain('<button type="submit" class="btn btn-primary btn-sm"');
    expect(general).not.toContain('AppButton');
    expect(general).toContain('project.settings_general_subtitle');
  });

  it('styles the read-only issuer labels alike (P-c) and hides the logo help when storage is off (P-f)', () => {
    const issuer = pages.find(p => p.name === 'issuer')!.source;
    expect(issuer).not.toContain('text-xs font-semibold');
    const branding = pages.find(p => p.name === 'branding')!.source;
    expect(branding).toMatch(/<p v-if="!storageOff" :id="ids\.logoHelp"/);
  });

  it('says the logo saves on upload, and gives the storage-off next step (product M1.5 1, 6)', () => {
    const branding = pages.find(p => p.name === 'branding')!.source;
    expect(branding).toContain('<SavedNote v-if="logoSaved" :label="t(\'branding.logo_updated\')" />');
    expect(branding).toContain('branding.logo_instant');
    expect(branding).toContain('branding.storage_off');
    expect(branding).not.toContain('apperror.STO003');
  });

  it('shows members a neutral line when no terms link is set (product M1.5 4)', () => {
    const legal = pages.find(p => p.name === 'legal')!.source;
    expect(legal).toContain('canEdit ? t(\'legal.no_terms\') : t(\'legal.no_terms_readonly\')');
  });

  it('uses one empty-value dash in the read-only views (P-b)', () => {
    for (const { name, source } of pages) {
      expect(source, name).not.toContain('–');
    }
  });

  it('lets issuer URLs wrap after /p/ (P-d) and calls the Overview link "Edit issuer" (P-e)', () => {
    const overview = read('../../pages/o/[org]/projects/[project]/index.vue');
    expect(overview).toContain('issuer.edit');
    expect(overview).toContain('<IssuerUrl');
    expect(pages.find(p => p.name === 'issuer')!.source).toContain('<IssuerUrl');
  });
});
